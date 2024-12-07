package monitor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/debrid"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

type sonarrEvent int

const (
	CreateOrWrite sonarrEvent = iota
	Unknown
)

type debounceEntry struct {
	timers map[sonarrEvent]*time.Timer
	mu     sync.Mutex
}

var debounceTimers sync.Map // A concurrent map to track timers for each file

type SonarrState int

const (
	New SonarrState = iota
	InProcessing
	WaitingFileSelection
	AddedToDebrid // TODO: Better name for this
	Complete
	Failed
)

// TODO: Handle state transition better
// TODO: More safe guards
// TODO: Better passing of values rather than some just being null all the time
// I think copilot could be good for a refactor after I write the function
// that injects an item already in the InProcessing state

// AddToDebrid -> SelectFile (Based on getInfo response)
type SonarrItem struct {
	InitialPath string
	State       SonarrState
	TorrentId   string
	Torrent     torrents.ToProcess
	err         error
}

// TODO: Some validation to make sure state changes, to prevent looping
func (s *SonarrItem) handle() {
	switch s.State {
	case New:
		s.onNew()
	case InProcessing:
		s.onProcessingItem()
	case AddedToDebrid:
		s.handleDebridState()
	case WaitingFileSelection:
		s.onWaitingForFileSelection()
	case Failed:
		s.onFailure()
	case Complete:
		s.onCompletion()
	}
}

func (s *SonarrItem) getBestFileName() string {
	if (s.Torrent != torrents.ToProcess{}) {
		return s.Torrent.FullPath
	} else if s.TorrentId != "" {
		return s.TorrentId
	} else if s.InitialPath != "" {
		return s.InitialPath
	}

	return "Unknown"
}

func (s *SonarrItem) onNew() {
	if s.InitialPath == "" {
		s.err = errors.New("Initial path is not set")
		s.State = Failed
		return
	}

	log.Printf("[sonarr]\t\tmoving %s to processing\n", s.InitialPath)
	sonarrConfig := config.GetAppConfig().Sonarr
	toProcess, err := torrents.NewFileToProcess(s.InitialPath, sonarrConfig.ProcessingPath)
	if err != nil {
		s.err = err
		s.State = Failed
		return
	}

	s.Torrent = toProcess
	s.State = InProcessing
}

func (s *SonarrItem) onProcessingItem() {
	if (s.Torrent == torrents.ToProcess{}) {
		s.err = errors.New("Torrent is not set")
		s.State = Failed
		return
	}

	switch s.Torrent.FileType {
	case torrents.TorrentFile:
		log.Printf("[sonarr]\t\tadding torrent file to debrid: %s\n", s.Torrent.FullPath)
		// TODO: Finish handling here - need to find a torrent file to test with
		torrentResponse, err := debrid.AddTorrent(s.Torrent.FullPath)
		if err != nil {
			s.err = err
			s.State = Failed
			return
		}
		s.TorrentId = torrentResponse.ID
	case torrents.Magnet:
		log.Printf("[sonarr]\t\tadding magnet to debrid: %s\n", s.Torrent.FullPath)
		magnetLink, err := s.Torrent.GetMagnetLink()
		if err != nil {
			s.err = err
			s.State = Failed
			return
		}

		magnetResponse, err := debrid.AddMagnet(magnetLink)
		if err != nil {
			s.err = err
			s.State = Failed
			return
		}

		s.TorrentId = magnetResponse.ID
	}

	s.State = AddedToDebrid
}

func (s *SonarrItem) handleDebridState() {
	if s.TorrentId == "" {
		s.err = errors.New("Missing debrid torrent ID")
		s.State = Failed
		return
	}

	torrentInfo, err := debrid.GetInfo(s.TorrentId)
	if err != nil {
		s.err = err
		s.State = Failed
		return
	}

	switch torrentInfo.Status {
	case debrid.WaitingFileSelection:
		s.State = WaitingFileSelection
		return
	case debrid.Downloading:
		// TODO: Flag this as only if instant_availabiltiy is desired
		// If it is downloading I will need something to periodically check debrid waiting for completion,
		// then can be moved to the other monitor
		s.err = errors.New("File is not instantly available")
		s.State = Failed
		return
	case debrid.Downloaded:
		log.Printf("[sonarr]\t\tadding to monitor: %s\n", torrentInfo.Filename)
		sonarrConfig := config.GetAppConfig().Sonarr
		MonitorForDebridFiles(MonitorConfig{
			Filename:         torrentInfo.Filename,
			OriginalFilename: torrentInfo.OriginalFilename,
			CompletedDir:     sonarrConfig.CompletedPath,
			Service:          arr.Sonarr,
			ProcessingPath:   s.Torrent.FullPath,
		})
		s.State = Complete
	default:
		s.err = errors.New(fmt.Sprintf("Unexpected debrid status - %s", torrentInfo.Status))
		s.State = Failed
		return
	}

}

func (s *SonarrItem) onWaitingForFileSelection() {
	if s.TorrentId == "" {
		s.err = errors.New("Missing debrid torrent ID")
		s.State = Failed
	}

	log.Printf("[sonarr]\t\tselecting all files for: %s\n", s.TorrentId)
	err := debrid.SelectFiles(s.TorrentId, []string{})
	if err != nil {
		s.err = err
		s.State = Failed
		return
	}

	s.State = AddedToDebrid
}

func (s *SonarrItem) onFailure() {
	// TODO: Notify sonarr of the failure
	log.Printf("[sonarr]\t\tencountered error: %s", s.err)
	log.Printf("[sonarr]\t\tunable to process %s - exiting", s.getBestFileName())
	s.State = Complete
}

func (s *SonarrItem) onCompletion() {
	log.Printf("[sonarr]\t\tfinished handling: %s", s.getBestFileName())
}

func sonarrEventFromFileEvent(e fsnotify.Event) sonarrEvent {
	switch e.Op {
	case fsnotify.Create:
	case fsnotify.Write:
		return CreateOrWrite
	}

	return Unknown
}

// SonarrMonitorHandler handles all the different events from the fsnotify
// file system watcher for a certain directory. These direcetories
// should be getting .torrent or .magnet files places in them by
// the *arr applications when setup using a Blackhole torrent client
func SonarrMonitorHandler(e fsnotify.Event, root string) {
	// NOTE: In earlier testing the name was onl the filename not the full path, this could be a linux/darwin difference
	// filepath := path.Join(root, e.Name)

	// Get or initialize a debounce entry for the file
	const debounceDuration = 5 * time.Second
	entry, _ := debounceTimers.LoadOrStore(e.Name, &debounceEntry{
		timers: make(map[sonarrEvent]*time.Timer),
	})

	debounce := entry.(*debounceEntry)
	debounce.mu.Lock()
	defer debounce.mu.Unlock()

	// Reset all the timers for this file
	for _, timer := range debounce.timers {
		// Stop and reset the timer
		if !timer.Stop() {
			<-timer.C // Drain channel to prevent leaks
		}
		timer.Reset(debounceDuration)
	}

	eventType := sonarrEventFromFileEvent(e)

	// Create a new timer for this event type, if doesn't exist
	if _, exists := debounce.timers[eventType]; !exists {
		timer := time.AfterFunc(debounceDuration, func() {
			handleEvent(eventType, e.Name)

			// Clean up the timer after execution
			debounce.mu.Lock()
			delete(debounce.timers, eventType)
			if len(debounce.timers) == 0 {
				// If no timers remain, remove the file entry entirely
				debounceTimers.Delete(e.Name)
			}
			debounce.mu.Unlock()
		})
		debounce.timers[eventType] = timer
	}
}

func handleEvent(e sonarrEvent, filepath string) {
	if _, err := os.Stat(filepath); err != nil {
		log.Printf("[sonarr]\t\tfile no longer exists: %s\n", filepath)
		return
	}

	switch e {
	case CreateOrWrite:
		handleNewSonarrFile(filepath)
	}
}

// handleNewSonarrFile handles when a new file is created in the watched
// directory. It will process the file, add it to debrid and then move to
// completed once it has been finished processing, then the *arr application
// can finish the handling
func handleNewSonarrFile(filepath string) {
	log.Printf("[sonarr]\t\tcreated file: %s\n", filepath)

	stateMachineItem := SonarrItem{
		InitialPath: filepath,
		State:       New,
	}

	ExecuteStateMachine(stateMachineItem)
}

func ExecuteStateMachine(item SonarrItem) {
	for item.State != Complete {
		item.handle()
	}
}
