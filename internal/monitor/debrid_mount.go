package monitor

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
)

// TODO: Will probably want to scan processing dirs
// on reboot and add to the monitored set

// TODO: Maybe debounce, so wait until there have been no events in that
// directory for a certain amount of time (this would allow everything to
// have been written)

// FIXME: make sure this works with directory
func DebridMountMonitorHandler(e watcher.Event, root string) {
	// NOTE: In earlier testing the name was onl the filename not the full path, this could be a linux/darwin difference
	// filepath := path.Join(root, e.Name)
	name := path.Base(e.Path)

	switch e.Op {
	case watcher.Create:
		handleNewFileInMount(e.Path, name)
	}
}

type MonitorConfig struct {
	Filename         string
	OriginalFilename string
	CompletedDir     string
	ProcessingPath   string
	Service          arr.ArrService
}

func MonitorForDebridFiles(c MonitorConfig) error {
	expectedPath := path.Join(config.GetAppConfig().RealDebrid.WatchPatch, c.Filename)

	if _, err := os.Stat(expectedPath); err == nil {
		log.Printf("[debrid-monitor]\t%s already exists, going to process", c.Filename)
		handleNewFileInMount(expectedPath, c.Filename)
		return nil
	}

	timeout := time.Duration(config.GetAppConfig().RealDebrid.MountTimeout) * time.Second
	expiry := time.Now().Add(timeout)
	log.Printf("[debrid-monitor]\tadding %s, watching until %v", c.Filename, expiry)
	pathSet := getInstance()
	meta := PathMeta{
		Expiration:       expiry,
		OriginalFileName: c.OriginalFilename,
		Service:          c.Service,
		CompletedDir:     c.CompletedDir,
		ProcessingPath:   c.ProcessingPath,
	}
	pathSet.add(c.Filename, meta)

	return nil
}

// TODO: on error handle properly making sure *arr knows there was an error
func handleNewFileInMount(filePath string, filename string) {
	pathSet := getInstance()
	// FIXME: Eventually also check the originalfilename, I guess
	pathMeta := pathSet.remove(filename)

	if (pathMeta == PathMeta{}) {
		log.Printf("[debrid-monitor]\tnot monitoring file: %s", filename)
		return
	}

	if _, err := os.Stat(pathMeta.ProcessingPath); err != nil {
		log.Printf("[debrid-monitor]\t%s doesn't exist anymore, not processing", pathMeta.ProcessingPath)
		return
	}

	log.Printf("[debrid-monitor]\tstarting linking of: %s", filename)

	completedPath := path.Join(pathMeta.CompletedDir, filename)
	os.Mkdir(completedPath, os.ModePerm)

	err := filepath.WalkDir(filePath, func(currentFile string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(filePath, currentFile)
		if err != nil {
			return err
		}

		currentFileToMove := path.Join(completedPath, relativePath)

		if strings.Contains(relativePath, "../") {
			return errors.New("File appears to be from outside root dir")
		}

		toMoveParentDir, _ := path.Split(currentFileToMove)
		err = os.MkdirAll(toMoveParentDir, os.ModePerm)
		if err != nil {
			return err
		}

		err = os.Symlink(currentFile, currentFileToMove)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		panic(err)
	}

	log.Printf("[debrid-monitor]\tnotifying %s processing complete of %s", pathMeta.Service.String(), filename)
	// TODO: error handles
	// TODO: Confirm refresh happened
	switch pathMeta.Service {
	case arr.Sonarr:
		arr.SonarrRefreshMonitoredDownloads()
	}
}
