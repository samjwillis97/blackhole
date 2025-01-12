package monitor

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/radovskyb/watcher"
)

// TODO: rename module

type Monitor struct {
	Logger   *slog.Logger
	Settings []MonitorSetting
}

type MonitorSetting struct {
	Name         string
	Directory    string
	EventHandler func(fsnotify.Event, string, *slog.Logger)
	PollHandler  func(watcher.Event, string, *slog.Logger)
}

func (m *Monitor) StartMonitoring() (*fsnotify.Watcher, *watcher.Watcher) {
	m.Logger.Info("initializing monitor")

	eventBasedWatcher, err := m.createEventBasedWatcher()
	if err != nil {
		m.Logger.Error("failed to create event based watcher", "err", err)
		panic(1)
	}

	pollBasedWatcher, err := m.createPollingBasedWatcher()
	if err != nil {
		m.Logger.Error("failed to create poll based watcher", "err", err)
		panic(1)
	}

	return eventBasedWatcher, pollBasedWatcher
}

func (m *Monitor) createPollingBasedWatcher() (*watcher.Watcher, error) {
	pollingBasedMonitors := []MonitorSetting{}
	for _, s := range m.Settings {
		if s.PollHandler != nil {
			pollingBasedMonitors = append(pollingBasedMonitors, s)
		}
	}

	logger := m.Logger.With("monitorType", "poll")

	// Create
	w := watcher.New()

	// SetMaxEvents to 1 to allow at most 1 event's to be received
	// on the Event channel per watching cycle.
	//
	// If SetMaxEvents is not set, the default is to send all events.
	w.SetMaxEvents(1)

	go m.pollWatchHandler(w, pollingBasedMonitors)

	for _, setting := range pollingBasedMonitors {
		logger.Info("watching directory", "directory", setting.Directory)
		err := w.Add(setting.Directory)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to watch %s: %s", setting.Directory, err))
		}
	}

	go w.Start(time.Second * 1)

	logger.Info("monitor started")

	return w, nil
}

func (m *Monitor) pollWatchHandler(w *watcher.Watcher, s []MonitorSetting) {
	logger := m.Logger.With("monitorType", "poll")
	for {
		select {
		case event := <-w.Event:
			for _, setting := range s {
				if strings.Contains(event.Path, setting.Directory) {
					eventId := uuid.New()
					logger = logger.With("monitorName", setting.Name).With("monitorEventType", event.Op.String()).With("monitorEventPath", event.Path).With("eventID", eventId)

					logger.Debug("event received")

					setting.PollHandler(event, setting.Directory, logger)
				}
			}
		case err := <-w.Error:
			logger.Error("monitor encountered error", "err", err)
			panic(1)
		case <-w.Closed:
			return
		}
	}
}

func (m *Monitor) createEventBasedWatcher() (*fsnotify.Watcher, error) {
	eventBasedMonitors := []MonitorSetting{}
	for _, s := range m.Settings {
		if s.EventHandler != nil {
			eventBasedMonitors = append(eventBasedMonitors, s)
		}
	}

	logger := m.Logger.With("monitorType", "event")

	// Create new event based watcher
	eventWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create event watcher: %s", err))
	}

	// Start listening for events.
	go eventWatchHandler(eventWatcher, eventBasedMonitors, logger)

	for _, setting := range eventBasedMonitors {
		logger.Info("watching", "directory", setting.Directory)
		err = eventWatcher.Add(setting.Directory)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to watch %s: %s", setting.Directory, err))
		}
	}

	logger.Info("monitor started")

	return eventWatcher, nil
}

func eventWatchHandler(w *fsnotify.Watcher, s []MonitorSetting, logger *slog.Logger) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				logger.Error("not okay event received")
				return
			}

			for _, setting := range s {
				if strings.Contains(event.Name, setting.Directory) {
					eventId := uuid.New()
					logger = logger.With("monitorName", setting.Name).With("monitorEventType", event.Op.String()).With("monitorEventPath", event.Name).With("eventID", eventId)

					logger.Debug("event received")
					// FIXME: I dont like putting a `go` here, feels like there is something blocking the function
					go setting.EventHandler(event, setting.Directory, logger)
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}

			logger.Error("monitor encountered error", "err", err)
		}
	}
}
