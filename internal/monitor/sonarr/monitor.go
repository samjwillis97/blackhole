package sonarr

import (
	"log/slog"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func MonitorHandlerBuilder(conf config.SonarrConfig) func(e fsnotify.Event, s string, l *slog.Logger) {
	return func(e fsnotify.Event, _ string, logger *slog.Logger) {
		switch e.Op {
		case fsnotify.Create:
		case fsnotify.Write:
			monitor.Debounce(e.Name, monitor.CreateOrWrite, func() {
				NewTorrentFile(conf, e.Name, logger)
			})
		}
	}
}
