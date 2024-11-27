package monitor

import (
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// TODO: rename module

type MonitorSetting struct {
	Name      string
	Directory string
	Handler   func(fsnotify.Event, string)
}

func StartMonitoring(settings []MonitorSetting) *fsnotify.Watcher {
	log.Println("[fs-watcher] initializing")

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
		// log.Fatal(err)
	}

	// Start listening for events.
	go watchHandler(watcher, settings)

	for _, setting := range settings {
		log.Printf("[fs-watcher] watching: %s", setting.Directory)
		err = watcher.Add(setting.Directory)
		if err != nil {
			panic(err)
		}
	}

	log.Printf("[fs-watcher] started")

	return watcher
}

func watchHandler(w *fsnotify.Watcher, s []MonitorSetting) {
	for {
		select {
		case event, ok := <-w.Events:
			log.Printf("[fs-watcher] %s event for %s", event.Op.String(), event.Name)
			if !ok {
				return
			}

			for _, setting := range s {
				if strings.Contains(event.Name, setting.Directory) {
					log.Printf("[fs-watcher] calling handler: %s", setting.Name)
					setting.Handler(event, setting.Directory)
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
