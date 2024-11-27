package debrid

import (
	"fmt"
	"path"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Want a set or something to keep track of wht we are waiting for
// this should really have a mutex/lock around it

// Will probably want to scan processing dirs
// on reboot and add to the monitored set

func DebridMountMonitorHandler(e fsnotify.Event, root string) {
	filepath := path.Join(root, e.Name)

	switch e.Op {
	case fsnotify.Create:
		handleNewFileInMount(filepath, e.Name)
	}
}

func MonitorForFiles(name string) error {
	pathSet := getInstance()
	pathSet.add(name, 60*time.Second)
	return nil
}

func handleNewFileInMount(filepath string, filename string) {
	pathSet := getInstance()

	if !pathSet.exists(filename) {
		fmt.Println("File not found in set")
	}

	// make dir in completed
}
