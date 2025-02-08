package main

import (
	"errors"
	"log/slog"
	"os"
	"path"

	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/logger"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/debrid"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/sonarr"
)

func main() {
	// logger.Main()
	log := slog.New(logger.NewHandler(&slog.HandlerOptions{Level: slog.LevelDebug}))
	log.Info("starting")

	monitorSetttings := []monitor.MonitorSetting{}

	monitorSetttings = append(monitorSetttings, setupSonarrMonitor(log)...)
	monitorSetttings = append(monitorSetttings, setupRadarrMonitor(log)...)
	monitorSetttings = append(monitorSetttings, setupDebridMonitor(log))

	monitorSetup := monitor.Monitor{
		Logger:   log,
		Settings: monitorSetttings,
	}

	eventWatcher, pollWatcher := monitorSetup.StartMonitoring()
	defer eventWatcher.Close()
	defer pollWatcher.Close()

	<-make(chan struct{})
}

func setupRadarrMonitor(log *slog.Logger) []monitor.MonitorSetting {

	monitors := []monitor.MonitorSetting{}

	for _, config := range config.GetAppConfig().Radarr {
		radarrFilesToResume, err := os.ReadDir(config.ProcessingPath)
		if err != nil {
			panic(errors.New("Failed to read radarr processing directory"))
		}

		log.Info("resuming processing of existing radarr files")
		for _, f := range radarrFilesToResume {
			if f.IsDir() {
				continue
			}

			pathToProcess := path.Join(config.ProcessingPath, f.Name())
			log.Info("resuming file", "file", pathToProcess)
			err := sonarr.ResumeProcessingFile(arr.Radarr, config, pathToProcess, log)
			if err != nil {
				log.Warn("processing failed", "file", pathToProcess, "err", err)
			}
		}

		currentRadarrFiles, err := os.ReadDir(config.WatchPath)
		if err != nil {
			panic(errors.New("Failed to read radarr monitor directory"))
		}

		log.Info("starting processing new radarr files")
		for _, f := range currentRadarrFiles {
			if f.IsDir() {
				continue
			}

			pathToProcess := path.Join(config.WatchPath, f.Name())
			log.Info("processing file", "file", pathToProcess)
			err := sonarr.NewTorrentFile(arr.Radarr, config, pathToProcess, log)
			if err != nil {
				log.Warn("processing failed", "file", pathToProcess, "err", err)
			}
		}

		log.Info("finished processing existing radarr files")

		monitors = append(monitors, monitor.MonitorSetting{
			Name:         config.Name,
			Directory:    config.WatchPath,
			EventHandler: sonarr.MonitorHandlerBuilder(arr.Radarr, config),
		},
		)
	}

	return monitors
}

func setupSonarrMonitor(log *slog.Logger) []monitor.MonitorSetting {

	monitors := []monitor.MonitorSetting{}

	for _, config := range config.GetAppConfig().Sonarr {
		sonarrFilesToResume, err := os.ReadDir(config.ProcessingPath)
		if err != nil {
			panic(errors.New("Failed to read sonarr processing directory"))
		}

		log.Info("resuming processing of existing sonarr files")
		for _, f := range sonarrFilesToResume {
			if f.IsDir() {
				continue
			}

			pathToProcess := path.Join(config.ProcessingPath, f.Name())
			log.Info("resuming file", "file", pathToProcess)
			err := sonarr.ResumeProcessingFile(arr.Sonarr, config, pathToProcess, log)
			if err != nil {
				log.Warn("processing failed", "file", pathToProcess, "err", err)
			}
		}

		currentSonarrFiles, err := os.ReadDir(config.WatchPath)
		if err != nil {
			panic(errors.New("Failed to read sonarr monitor directory"))
		}

		log.Info("starting processing new sonarr files")
		for _, f := range currentSonarrFiles {
			if f.IsDir() {
				continue
			}

			pathToProcess := path.Join(config.WatchPath, f.Name())
			log.Info("processing file", "file", pathToProcess)
			err := sonarr.NewTorrentFile(arr.Sonarr, config, pathToProcess, log)
			if err != nil {
				log.Warn("processing failed", "file", pathToProcess, "err", err)
			}
		}

		log.Info("finished processing existing sonarr files")

		monitors = append(monitors, monitor.MonitorSetting{
			Name:         config.Name,
			Directory:    config.WatchPath,
			EventHandler: sonarr.MonitorHandlerBuilder(arr.Sonarr, config),
		},
		)
	}

	return monitors
}

func setupDebridMonitor(log *slog.Logger) monitor.MonitorSetting {
	debridMonitorPath := config.GetAppConfig().RealDebrid.WatchPatch
	currentDebridFiles, err := os.ReadDir(debridMonitorPath)
	if err != nil {
		panic(errors.New("Failed to read debrid watch directory"))
	}

	log.Info("starting processing existing debrid files")
	for _, f := range currentDebridFiles {
		debrid.MonitorHandler(watcher.Event{
			Path: path.Join(debridMonitorPath, f.Name()),
			Op:   watcher.Create,
		}, debridMonitorPath, log)
	}
	log.Info("finished processing existing debrid files")

	return monitor.MonitorSetting{
		Name:        "Debrid Monitor",
		Directory:   debridMonitorPath,
		PollHandler: debrid.MonitorHandler,
	}
}
