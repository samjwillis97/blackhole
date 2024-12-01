package monitor

import (
	"log"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
)

// TODO: rename module

type MonitorSetting struct {
	Name         string
	Directory    string
	EventHandler func(fsnotify.Event, string)
	PollHandler  func(watcher.Event, string)
}

func StartMonitoring(settings []MonitorSetting) (*fsnotify.Watcher, *watcher.Watcher) {
	log.Println("[fs-watcher]\tinitializing")

	eventBasedMonitors := []MonitorSetting{}
	for _, s := range settings {
		if s.EventHandler != nil {
			eventBasedMonitors = append(eventBasedMonitors, s)
		}
	}

	pollingBasedMonitors := []MonitorSetting{}
	for _, s := range settings {
		if s.PollHandler != nil {
			pollingBasedMonitors = append(pollingBasedMonitors, s)
		}
	}

	eventBasedWatcher := createEventBasedWatcher(eventBasedMonitors)
	pollBasedWatcher := createPollingBasedWatcher(pollingBasedMonitors)

	return eventBasedWatcher, pollBasedWatcher
}

func createPollingBasedWatcher(settings []MonitorSetting) *watcher.Watcher {
	// Create
	w := watcher.New()

	// SetMaxEvents to 1 to allow at most 1 event's to be received
	// on the Event channel per watching cycle.
	//
	// If SetMaxEvents is not set, the default is to send all events.
	w.SetMaxEvents(1)

	go pollWatchHandler(w, settings)

	for _, setting := range settings {
		log.Printf("[fs-poll-watcher]\twatching: %s", setting.Directory)
		err := w.Add(setting.Directory)
		if err != nil {
			panic(err)
		}
	}

	go w.Start(time.Second * 1)

	log.Printf("[fs-poll-watcher]\tstarted")

	return w
}

func pollWatchHandler(w *watcher.Watcher, s []MonitorSetting) {
	for {
		select {
		case event := <-w.Event:
			for _, setting := range s {
				if strings.Contains(event.Path, setting.Directory) {
					log.Printf("[fs-poll-watcher]\t%s event for %s -> %s", event.Op.String(), event.Path, setting.Name)
					setting.PollHandler(event, setting.Directory)
				}
			}
		case err := <-w.Error:
			panic(err)
		case <-w.Closed:
			return
		}
	}
}

func createEventBasedWatcher(settings []MonitorSetting) *fsnotify.Watcher {
	// Create new event based watcher
	eventWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
		// log.Fatal(err)
	}

	// Start listening for events.
	go eventWatchHandler(eventWatcher, settings)

	for _, setting := range settings {
		log.Printf("[fs-event-watcher]\twatching: %s", setting.Directory)
		err = eventWatcher.Add(setting.Directory)
		if err != nil {
			panic(err)
		}
	}

	log.Printf("[fs-event-watcher]\tstarted")

	return eventWatcher
}

func eventWatchHandler(w *fsnotify.Watcher, s []MonitorSetting) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}

			for _, setting := range s {
				if strings.Contains(event.Name, setting.Directory) {
					log.Printf("[fs-event-watcher]\t%s event for %s -> %s", event.Op.String(), event.Name, setting.Name)
					setting.EventHandler(event, setting.Directory)
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
