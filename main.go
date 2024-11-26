package main

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

func main() {
	fmt.Println("Starting up")

	config.GetSecrets(nil)
	fmt.Println(config.GetAppConfig(nil))

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
