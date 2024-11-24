package monitor

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type MonitorSetting struct {
	Directory string
	Handler   func(fsnotify.Event)
}

func StartMonitoring(settings []MonitorSetting) *fsnotify.Watcher {
	fmt.Println("Initialising monitor")

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
		// log.Fatal(err)
	}

	// Start listening for events.
	go watchHandler(watcher, settings)

	for _, setting := range settings {
		fmt.Printf("Watching %s\n", setting.Directory)
		err = watcher.Add(setting.Directory)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Monitor setup")

	return watcher
}

func watchHandler(w *fsnotify.Watcher, s []MonitorSetting) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}

			for _, setting := range s {
				if strings.Contains(event.Name, setting.Directory) {
					setting.Handler(event)
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}

			panic(err)
			// log.Println("error:", err)
		}
	}
}
