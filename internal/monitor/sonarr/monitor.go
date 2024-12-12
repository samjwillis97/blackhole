package sonarr

import (
	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

const (
	loggerName = "sonarr-monitor"
)

func MonitorHandler(e fsnotify.Event, _ string) {
	switch e.Op {
	case fsnotify.Create:
	case fsnotify.Write:
		monitor.Debounce(e.Name, monitor.CreateOrWrite, func() {
			NewTorrentFile(e.Name)
		})
	}
}
