package main

import (
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func main() {
	log.Println("[app] Starting up")

	config.GetSecrets()

	// TODO: Also scan the folder for if files were added whilst not monitoring
	monitorSetup := []monitor.MonitorSetting{}

	monitorSetup = append(monitorSetup, monitor.MonitorSetting{
		Directory: "/Users/sam/code/github.com/samjwillis97/sams-blackhole/main",
		Handler: func(event fsnotify.Event, s string) {
			println("HELLLOO")
			println(event.Name)
			println(event.Op.String())
		},
	})

	w := monitor.StartMonitoring(monitorSetup)
	defer w.Close()

	<-make(chan struct{})
}
