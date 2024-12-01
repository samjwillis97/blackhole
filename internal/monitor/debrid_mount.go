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

// TODO: also check if it is already availabe here
func MonitorForFiles(name string, completedDir string, service arr.ArrService) error {
	timeout := time.Duration(config.GetAppConfig().RealDebrid.MountTimeout) * time.Second
	expiry := time.Now().Add(timeout)
	log.Printf("[debrid-monitor]\tadding %s, watching until %v", name, expiry)
	pathSet := getInstance()
	meta := PathMeta{
		Expiration:   expiry,
		Service:      service,
		CompletedDir: completedDir,
	}
	pathSet.add(name, meta)

	return nil
}

// TODO: on error handle properly making sure *arr knows there was an error
func handleNewFileInMount(filePath string, filename string) {
	pathSet := getInstance()
	pathMeta := pathSet.remove(filename)

	// Should get and existss in single call with the lock
	if (pathMeta == PathMeta{}) {
		log.Printf("[debrid-monitor]\tnot monitoring file: %s", filename)
		return
	}

	// TODO: Should we abort here if the file is no longer in processing? i.e. deleted, need to check what sonarr does

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
	switch pathMeta.Service {
	case arr.Sonarr:
		arr.SonarrRefreshMonitoredDownloads()
	}
}
