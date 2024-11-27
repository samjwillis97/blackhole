package monitor

import (
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type MonitorSetting struct {
	Name      string
	Directory string
	Handler   func(fsnotify.Event, string)
}

func StartMonitoring(settings []MonitorSetting) *fsnotify.Watcher {
	log.Println("[monitor] initializing")

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
		// log.Fatal(err)
	}

	// Start listening for events.
	go watchHandler(watcher, settings)

	for _, setting := range settings {
		log.Printf("[monitor] watching: %s", setting.Directory)
		err = watcher.Add(setting.Directory)
		if err != nil {
			panic(err)
		}
	}

	log.Printf("[monitor] started")

	return watcher
}

func watchHandler(w *fsnotify.Watcher, s []MonitorSetting) {
	for {
		select {
		case event, ok := <-w.Events:
			log.Printf("[monitor] %s event for %s", event.Op.String(), event.Name)
			if !ok {
				return
			}

			for _, setting := range s {
				if strings.Contains(event.Name, setting.Directory) {
					log.Printf("[monitor] calling handler: %s", setting.Name)
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
