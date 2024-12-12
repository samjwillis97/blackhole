package main

import (
	"errors"
	"log"
	"os"
	"path"

	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/sonarr"
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
	sonarrProcessingPath := config.GetAppConfig().Sonarr.ProcessingPath
	sonarrFilesToResume, err := os.ReadDir(sonarrProcessingPath)
	if err != nil {
		panic(errors.New("Failed to read sonarr processing directory"))
	}

	log.Println("[app]\t\tresuming processing existing sonarr files")
	for _, f := range sonarrFilesToResume {
		if f.IsDir() {
			continue
		}

		pathToProcess := path.Join(sonarrProcessingPath, f.Name())
		log.Printf("[app]\t\tresuming %s", pathToProcess)
		err := sonarr.ResumeProcessingFile(pathToProcess)
		if err != nil {
			log.Printf("Failed to process %s: %s", pathToProcess, err)
		}
	}

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

		pathToProcess := path.Join(sonarrMonitorPath, f.Name())
		log.Printf("[app]\t\tprocessing %s", pathToProcess)
		err := sonarr.NewTorrentFile(pathToProcess)
		if err != nil {
			log.Printf("Failed to process %s: %s", pathToProcess, err)
		}
	}

	log.Println("[app]\t\tfinished processing existing sonarr files")

	return monitor.MonitorSetting{
		Name:         "Sonarr Monitor",
		Directory:    sonarrMonitorPath,
		EventHandler: sonarr.MonitorHandler,
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
