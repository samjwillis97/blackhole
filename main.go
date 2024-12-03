package main

import (
	"errors"
	"log"
	"os"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func main() {
	log.Println("[app]\t\tstarting")

	monitorSetup := []monitor.MonitorSetting{}

	monitorSetup = append(monitorSetup, setupSonarrMonitor())
	monitorSetup = append(monitorSetup, setupDebridMonitor())

	eventWatcher, pollWatcher := monitor.StartMonitoring(monitorSetup)
	defer eventWatcher.Close()
	defer pollWatcher.Close()

	<-make(chan struct{})
}

func setupSonarrMonitor() monitor.MonitorSetting {
	sonarrMonitorPath := config.GetAppConfig().Sonarr.WatchPath
	currentSonarrFiles, err := os.ReadDir(sonarrMonitorPath)
	if err != nil {
		panic(errors.New("Failed to read sonarr monitor directory"))
	}

	log.Println("[app]\t\tstarting processing existing sonarr files")
	for _, f := range currentSonarrFiles {
		if f.IsDir() {
			continue
		}

		monitor.SonarrMonitorHandler(fsnotify.Event{
			Name: path.Join(sonarrMonitorPath, f.Name()),
			Op:   fsnotify.Create,
		}, sonarrMonitorPath)
	}
	log.Println("[app]\t\tfinished processing existing sonarr files")

	return monitor.MonitorSetting{
		Name:         "Sonarr Monitor",
		Directory:    sonarrMonitorPath,
		EventHandler: monitor.SonarrMonitorHandler,
	}
}

func setupDebridMonitor() monitor.MonitorSetting {
	debridMonitorPath := config.GetAppConfig().RealDebrid.WatchPatch
	currentDebridFiles, err := os.ReadDir(debridMonitorPath)
	if err != nil {
		panic(errors.New("Failed to read debrid watch directory"))
	}

	log.Println("[app]\t\tstarting processing existing debrid files")
	for _, f := range currentDebridFiles {
		monitor.DebridMountMonitorHandler(watcher.Event{
			Path: path.Join(debridMonitorPath, f.Name()),
			Op:   watcher.Create,
		}, debridMonitorPath)
	}
	log.Println("[app]\t\tfinished processing existing debrid files")

	return monitor.MonitorSetting{
		Name:        "Debrid Monitor",
		Directory:   debridMonitorPath,
		PollHandler: monitor.DebridMountMonitorHandler,
	}
}
