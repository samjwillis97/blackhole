package monitor

import (
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

func sonarrEventFromFileEvent(e fsnotify.Event) sonarrEvent {
	switch e.Op {
	case fsnotify.Create:
	case fsnotify.Write:
		return CreateOrWrite
	}

	return Unknown
}

type debounceEntry struct {
	timers map[sonarrEvent]*time.Timer
	mu     sync.Mutex
}

var debounceTimers sync.Map // A concurrent map to track timers for each file

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
		panic(err)
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

	sonarrConfig := config.GetAppConfig().Sonarr

	toProcess, err := torrents.NewFileToProcess(filepath, sonarrConfig.ProcessingPath)
	if err != nil {
		panic(err)
	}

	var torrentId string
	switch toProcess.FileType {
	case torrents.TorrentFile:
		log.Printf("[sonarr]\t\tadding torrent file to debrid: %s\n", filepath)
		// TODO: Finish handling here - need to find a torrent file to test with
		debrid.AddTorrent(toProcess.FullPath)
	case torrents.Magnet:
		log.Printf("[sonarr]\t\tadding magnet to debrid: %s\n", filepath)
		magnetResponse, err := debrid.AddMagnet(toProcess.FullPath)
		if err != nil {
			log.Printf("[sonarr]\t\tencountered error: %s", err)
			log.Printf("[sonarr]\t\tunable to process %s - exiting", filepath)
			return
		}

		torrentId = magnetResponse.ID
		log.Printf("[sonarr]\t\tselect all files for: %s\n", magnetResponse.ID)
		// NOTE: this could be derived from the getInfo, it has a status to say it is waiting for file selection
		// this can also be used to show downloading failed etc.
		// Could move this down below into a state machine on a timer with a max time
		err = debrid.SelectFiles(magnetResponse.ID, []string{})
		if err != nil {
			log.Printf("[sonarr]\t\tencountered error: %s", err)
			log.Printf("[sonarr]\t\tunable to process %s - exiting", magnetResponse.ID)
			return
		}
	}

	log.Printf("[sonarr]\t\tGetting torrent info for: %s\n", torrentId)
	torrentInfo, err := debrid.GetInfo(torrentId)
	if err != nil {
		log.Printf("[sonarr]\t\tencountered error: %s", err)
		log.Printf("[sonarr]\t\tunable to process %s - exiting", torrentId)
		return
	}

	log.Printf("[sonarr]\t\tadding to monitor: %s\n", torrentInfo.Filename)
	err = MonitorForDebridFiles(MonitorConfig{
		Filename:         torrentInfo.Filename,
		OriginalFilename: torrentInfo.OriginalFilename,
		CompletedDir:     sonarrConfig.CompletedPath,
		Service:          arr.Sonarr,
		ProcessingPath:   toProcess.FullPath,
	})
  if err != nil {
		log.Printf("[sonarr]\t\tencountered error: %s", err)
		log.Printf("[sonarr]\t\tunable to process %s - exiting", torrentId)
		return
  }
	log.Printf("[sonarr]\t\tfinished handling: %s", toProcess.FilenameNoExt)
}
