package main

import (
	"log"

	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func main() {
	log.Println("[app] starting")

	// TODO: Also scan the folder for if files were added whilst not monitoring
	monitorSetup := []monitor.MonitorSetting{}

	monitorSetup = append(monitorSetup, monitor.MonitorSetting{
		Name:         "Sonarr Monitor",
		Directory:    config.GetAppConfig().Sonarr.WatchPath,
		EventHandler: monitor.SonarrMonitorHandler,
	})

	monitorSetup = append(monitorSetup, monitor.MonitorSetting{
		Name:        "Debrid Monitor",
		Directory:   config.GetAppConfig().RealDebrid.WatchPatch,
		PollHandler: monitor.DebridMountMonitorHandler,
	})

	eventWatcher, pollWatcher := monitor.StartMonitoring(monitorSetup)
	defer eventWatcher.Close()
	defer pollWatcher.Close()

	<-make(chan struct{})
}
