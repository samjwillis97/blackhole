package sonarr

import (
	"log/slog"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func MonitorHandler(e fsnotify.Event, _ string, logger *slog.Logger) {
	switch e.Op {
	case fsnotify.Create:
	case fsnotify.Write:
		monitor.Debounce(e.Name, monitor.CreateOrWrite, func() {
			NewTorrentFile(e.Name, logger)
		})
	}
}
