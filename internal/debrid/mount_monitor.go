package debrid

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
)

// TODO: Will probably want to scan processing dirs
// on reboot and add to the monitored set

func DebridMountMonitorHandler(e fsnotify.Event, root string) {
	filepath := path.Join(root, e.Name)

	switch e.Op {
	case fsnotify.Create:
		handleNewFileInMount(filepath, e.Name)
	}
}

func MonitorForFiles(name string, completedDir string) error {
	log.Printf("[debrid-monitor]\tadding %s", name)
	timeout := time.Duration(config.GetAppConfig().RealDebrid.MountTimeout) * time.Second
	pathSet := getInstance()
	meta := PathMeta{
		Expiration:   time.Now().Add(timeout),
		CompletedDir: completedDir,
	}
	pathSet.add(name, meta)

	return nil
}

func GetMonitoredFile(name string) PathMeta {
	pathSet := getInstance()
	return pathSet.get(name)
}

func handleNewFileInMount(_ string, filename string) {
	pathSet := getInstance()

	if !pathSet.exists(filename) {
		fmt.Println("File not found in set")
	}

	// make dir in completed
}
