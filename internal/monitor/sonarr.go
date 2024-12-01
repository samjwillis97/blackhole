package monitor

import (
	"log"
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
	// TODO: Because of all the debouncing, should check if the file still exists
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

	// fmt.Println(toProcess)

	log.Printf("[sonarr]\t\tadding to debrid: %s\n", filepath)
	switch toProcess.FileType {
	case torrents.TorrentFile:
		debrid.AddTorrent(toProcess.FullPath)
	case torrents.Magnet:
		magnetResponse := debrid.AddMagnet(toProcess.FullPath)
		log.Printf("[sonarr]\t\tselect all files for: %s\n", magnetResponse.ID)
		debrid.SelectFiles(magnetResponse.ID, []string{})
	}

	log.Printf("[sonarr]\t\tadding to monitor: %s\n", toProcess.FilenameNoExt)
	MonitorForFiles(toProcess.FilenameNoExt, sonarrConfig.CompletedPath, arr.Sonarr)

	// There will be some different handling dependant on whether
	// it should be avaiable on "instant_availability" endpoint (dead on debrid)
	// As well as if there is only a single item in the torrent, i Think this can be ignored though and let *arr handle

}
