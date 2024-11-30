package monitor

import (
	"log"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
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
	log.Printf("[sonarr]\t\tcreated file: %s\n", filepath)

	sonarrConfig := config.GetAppConfig().Sonarr

	toProcess, err := torrents.NewFileToProcess(filepath, sonarrConfig.ProcessingPath)
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

	MonitorForFiles(toProcess.FilenameNoExt, sonarrConfig.CompletedPath, arr.Sonarr)

	// There will be some different handling dependant on whether
	// it should be avaiable on "instant_availability" endpoint (dead on debrid)
	// As well as if there is only a single item in the torrent, i Think this can be ignored though and let *arr handle

}
