package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
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
	startTime   time.Time
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
	// Should probably just store the name to start of the machine
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
	// Requires:
	//   - current magnet/torrent path
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
	// Requires
	//   - torent in processing
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
		log.Printf("[sonarr]\t\tgetting magnet link for: %s\n", s.Torrent.FullPath)
		magnetLink, err := s.Torrent.GetMagnetLink()
		if err != nil {
			s.err = err
			s.State = Failed
			return
		}

		log.Printf("[sonarr]\t\tadding magnet to debrid: %s\n", s.Torrent.FullPath)
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
	// Requires
	//  - debrid torrent ID
	//  - processing path
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

	log.Printf("[sonarr]\t\tdebrid status of %s = %s", s.getBestFileName(), torrentInfo.Status)
	switch torrentInfo.Status {
	case debrid.WaitingFileSelection:
		s.State = WaitingFileSelection
		return
	case debrid.Queued:
		time.Sleep(1 * time.Second)
		return
	case debrid.Downloading:
		// TODO: Flag this as only if instant_availabiltiy is desired
		// If it is downloading I will need something to periodically check debrid waiting for completion,
		// then can be moved to the other monitor
		s.err = errors.New(fmt.Sprintf("%s is not instantly available", s.getBestFileName()))
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
	// Requires:
	//  - debrid torrent ID
	if s.TorrentId == "" {
		s.err = errors.New("Missing debrid torrent ID")
		s.State = Failed
	}

	log.Printf("[sonarr]\t\tselecting all files for: %s - %s\n", s.TorrentId, s.getBestFileName())
	err := debrid.SelectFiles(s.TorrentId, []string{})
	if err != nil {
		s.err = err
		s.State = Failed
		return
	}

	s.State = AddedToDebrid
}

func (s *SonarrItem) onFailure() {
	// Requires:
	//  - error message
	//  - torrent ID
	//  - processing path
	//  - magnet hash
	log.Printf("[sonarr]\t\tencountered error: %s", s.err)
	log.Printf("[sonarr]\t\tunable to process %s - removing", s.getBestFileName())

	if s.TorrentId != "" {
		err := debrid.Remove(s.TorrentId)
		if err != nil {
			log.Printf("[sonarr]\t\tfailed to remove from debrid: %s - %s", s.getBestFileName(), err)
		}
		log.Printf("[sonarr]\t\tsuccessfully removed %s from debrid", s.getBestFileName())
	}

	err := os.Remove(s.Torrent.FullPath)
	if err != nil {
		log.Printf("[sonarr]\t\tfailed to remove from processing: %s", s.getBestFileName())
	}

	s.removeFromSonarr()
	log.Printf("[sonarr]\t\tsuccessfully removed %s from sonarr", s.getBestFileName())

	// TODO: remove from blocklist after sometime.. not sure how to manage this

	s.State = Complete
}

func (s *SonarrItem) onCompletion() {
	log.Printf("[sonarr]\t\tfinished handling: %s", s.getBestFileName())
}

func (s *SonarrItem) removeFromSonarr() {
	hash, err := s.Torrent.GetMagnetHash()
	if err != nil {
		log.Printf("[sonarr]\t\tfailed to get hash for: %s", s.getBestFileName())
		return
	}

	history, err := arr.SonarrGetHistory(50)
	if err != nil {
		log.Printf("[sonarr]\t\tfailed to get history for: %s", s.getBestFileName())
		return
	}

	toRemove := slices.IndexFunc(history.Records, func(item arr.SonarrHistoryItem) bool {
		return item.EventType == arr.Grabbed && item.Data.TorrentInfoHash == hash
	})
	if toRemove == -1 {
		log.Printf("[sonarr]\t\tcould not find hash %s in history for %s", hash, s.getBestFileName())
		return
	}

	err = arr.SonarrFailHistoryItem(history.Records[toRemove].ID)
	if toRemove == -1 {
		log.Printf("[sonarr]\t\tfailed to fail history item with id %d - %s", history.Records[toRemove].ID, s.getBestFileName())
		return
	}
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

	sonarrTorrent := NewSonarrTorrent()
	sonarrTorrent.IngestedPath = filepath

	if err := sonarrTorrent.FSM.Event(context.Background(), "torrentFound"); err != nil {
		log.Printf("Failed to process %s: %s", filepath, err)
	}

	// stateMachineItem := SonarrItem{
	// 	InitialPath: filepath,
	// 	State:       New,
	// }
	//
	// ExecuteStateMachine(stateMachineItem)
}

func ExecuteStateMachine(item SonarrItem) {
	timeoutDuration := 30 * time.Second

	item.startTime = time.Now()
	for item.State != Complete {
		if time.Now().After(item.startTime.Add(timeoutDuration)) {
			item.err = errors.New("Timed out")
			item.State = Failed
		}
		// Check if timed out, if it has error out
		item.handle()
	}
}
