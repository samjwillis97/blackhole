package main

import (
	"errors"
	"log/slog"
	"os"
	"path"

	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/logger"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/sonarr"
)

func main() {
	logger.Main()
	// log := slog.New(logger.NewHandler(&slog.HandlerOptions{Level: slog.LevelDebug}))
	// log.Info("starting")
	//
	// monitorSetttings := []monitor.MonitorSetting{}
	//
	// monitorSetttings = append(monitorSetttings, setupSonarrMonitor(log))
	// monitorSetttings = append(monitorSetttings, setupDebridMonitor(log))
	//
	// monitorSetup := monitor.Monitor{
	// 	Logger:   log,
	// 	Settings: monitorSetttings,
	// }
	//
	// eventWatcher, pollWatcher := monitorSetup.StartMonitoring()
	// defer eventWatcher.Close()
	// defer pollWatcher.Close()
	//
	// <-make(chan struct{})
}

func setupSonarrMonitor(log *slog.Logger) monitor.MonitorSetting {
	sonarrProcessingPath := config.GetAppConfig().Sonarr.ProcessingPath
	sonarrFilesToResume, err := os.ReadDir(sonarrProcessingPath)
	if err != nil {
		panic(errors.New("Failed to read sonarr processing directory"))
	}

	log.Info("resuming processing of existing sonarr files")
	for _, f := range sonarrFilesToResume {
		if f.IsDir() {
			continue
		}

		pathToProcess := path.Join(sonarrProcessingPath, f.Name())
		log.Info("resuming file", "file", pathToProcess)
		err := sonarr.ResumeProcessingFile(pathToProcess, log)
		if err != nil {
			log.Warn("processing failed", "file", pathToProcess, "err", err)
		}
	}

	sonarrMonitorPath := config.GetAppConfig().Sonarr.WatchPath
	currentSonarrFiles, err := os.ReadDir(sonarrMonitorPath)
	if err != nil {
		panic(errors.New("Failed to read sonarr monitor directory"))
	}

	log.Info("starting processing new sonarr files")
	for _, f := range currentSonarrFiles {
		if f.IsDir() {
			continue
		}

		pathToProcess := path.Join(sonarrMonitorPath, f.Name())
		log.Info("processing file", "file", pathToProcess)
		err := sonarr.NewTorrentFile(pathToProcess, log)
		if err != nil {
			log.Warn("processing failed", "file", pathToProcess, "err", err)
		}
	}

	log.Info("finished processing existing sonarr files")

	return monitor.MonitorSetting{
		Name:         "Sonarr Monitor",
		Directory:    sonarrMonitorPath,
		EventHandler: sonarr.MonitorHandler,
	}
}

func setupDebridMonitor(log *slog.Logger) monitor.MonitorSetting {
	debridMonitorPath := config.GetAppConfig().RealDebrid.WatchPatch
	currentDebridFiles, err := os.ReadDir(debridMonitorPath)
	if err != nil {
		panic(errors.New("Failed to read debrid watch directory"))
	}

	log.Info("starting processing existing debrid files")
	for _, f := range currentDebridFiles {
		monitor.DebridMountMonitorHandler(watcher.Event{
			Path: path.Join(debridMonitorPath, f.Name()),
			Op:   watcher.Create,
		}, debridMonitorPath)
	}
	log.Info("finished processing existing debrid files")

	return monitor.MonitorSetting{
		Name:        "Debrid Monitor",
		Directory:   debridMonitorPath,
		PollHandler: monitor.DebridMountMonitorHandler,
	}
}
