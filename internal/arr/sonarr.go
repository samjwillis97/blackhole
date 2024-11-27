package arr

import (
	"fmt"
	"log"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/debrid"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

// SonarrMonitorHandler handles all the different events from the fsnotify
// file system watcher for a certain directory. These direcetories
// should be getting .torrent or .magnet files places in them by
// the *arr applications when setup using a Blackhole torrent client
func SonarrMonitorHandler(e fsnotify.Event, root string) {
	filepath := path.Join(root, e.Name)
	switch e.Op {
	case fsnotify.Create:
		handleNewSonarrFile(filepath)
	}
}

// handleNewSonarrFile handles when a new file is created in the watched
// directory. It will process the file, add it to debrid and then move to
// completed once it has been finished processing, then the *arr application
// can finish the handling
func handleNewSonarrFile(filepath string) {
	log.Printf("Handling created sonarr file: %s\n", filepath)

	processingLocation := config.GetAppConfig().Sonarr.ProcessingPath

	toProcess, err := torrents.NewFileToProcess(filepath, processingLocation)
	if err != nil {
		panic(err)
	}

	// fmt.Println(toProcess)

	switch toProcess.FileType {
	case torrents.TorrentFile:
		debrid.AddTorrent(toProcess.FullPath)
	case torrents.Magnet:
		debrid.AddMagnet(toProcess.FullPath)
	}

	debrid.MonitorForFiles(toProcess.Filename)

	// TODO: Handle waiting after torrent has been added
	// Thiswill probably require watching the mount or osmething like
	// that until a file turns up matching
	// Implementation:
	// Maybe create a watcher in main for the mount
	// Add files from here to a map/set, every time file appears in
	// mount, check if it is in the map/set, every 60s or whatever
	// the wait time is, clear out the map/set, and handle the
	// processing failed case

	// There will be some different handling dependant on whether
	// it should be avaiable on "instant_availability" endpoint (dead on debrid)
	// As well as if there is only a single item in the torrent, i Think this can be ignored though and let *arr handle

	// TODO: Attempt to add torrent to real-debrid
	// There will be slight differences in handling magnet files and .torrent files
	// due to different endpoints
}
