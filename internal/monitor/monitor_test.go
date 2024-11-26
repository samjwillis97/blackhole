package monitor_test

import (
	"os"
	"path"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

type result struct {
	bool
	fsnotify.Op
}

func TestBasicHandler(t *testing.T) {
	resultChannel := make(chan result)

	dir := os.TempDir()
	fileToCreate := path.Join(dir, "TestFile")

	_, err := os.Stat(fileToCreate)
	if err != nil {
		os.Remove(fileToCreate)
	}

	// Create settings
	settings := []monitor.MonitorSetting{}

	settings = append(settings, monitor.MonitorSetting{
		Directory: dir,
		Handler: func(e fsnotify.Event, s string) {
			resultChannel <- result{
				true,
				fsnotify.Create,
			}
		},
	})

	// Need to get a signal back for when the monitor has started
	w := monitor.StartMonitoring(settings)
	defer w.Close()

	// Then create a file or delete in the testing directory
	os.Create(fileToCreate)

	outcome := <-resultChannel
	if !outcome.bool || outcome.Op != fsnotify.Create {
		t.Errorf("Expected true create, received %t, %s", outcome.bool, outcome.Op.String())
	}
}

// TODO: Advanced test with atleast like a subdirectory
