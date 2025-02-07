package monitor_test

import (
	"log/slog"
	"os"
	"path"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/logger"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

type result struct {
	bool
	fsnotify.Op
}

// TODO: Given When Then
func TestBasicHandler(t *testing.T) {
	log := slog.New(logger.NewHandler(&slog.HandlerOptions{Level: slog.LevelDebug}))
	resultChannel := make(chan result)

	dir, _ := os.MkdirTemp("", "base-monitor-test-*")
	fileToCreate := path.Join(dir, "TestFile")

	_, err := os.Stat(fileToCreate)
	if err != nil {
		os.Remove(fileToCreate)
	}

	// Create settings
	settings := []monitor.MonitorSetting{}

	settings = append(settings, monitor.MonitorSetting{
		Name:      "test handler",
		Directory: dir,
		EventHandler: func(e fsnotify.Event, s string, log *slog.Logger) {
			resultChannel <- result{
				true,
				fsnotify.Create,
			}
		},
	})

	monitorSetup := monitor.Monitor{
		Logger:   log,
		Settings: settings,
	}

	// Need to get a signal back for when the monitor has started
	w, _ := monitorSetup.StartMonitoring()
	defer w.Close()

	// Then create a file or delete in the testing directory
	os.Create(fileToCreate)

	outcome := <-resultChannel
	if !outcome.bool || outcome.Op != fsnotify.Create {
		t.Errorf("Expected true create, received %t, %s", outcome.bool, outcome.Op.String())
	}
}

// TODO: Advanced test with atleast like a subdirectory
// dir, _ := os.MkdirTemp("", "monitor-test-*")
//  firstDir := path.Join(dir, "first")
// secon:= path.Join(dir, "TestFile")
// fileToCreate := path.Join(dir, "TestFile")

func TestMultipleEventHandlers(t *testing.T) {
	log := slog.New(logger.NewHandler(&slog.HandlerOptions{Level: slog.LevelDebug}))
	resultChannel := make(chan result)

	dir, _ := os.MkdirTemp("", "multiple-monitor-test-*")
	firstDir := path.Join(dir, "first")
	secondDir := path.Join(dir, "first 4k")

	os.Mkdir(firstDir, os.ModePerm)
	os.Mkdir(secondDir, os.ModePerm)

	fileToCreate := path.Join(secondDir, "TestFile")

	_, err := os.Stat(fileToCreate)
	if err != nil {
		os.Remove(fileToCreate)
	}

	// Create settings
	settings := []monitor.MonitorSetting{}

	settings = append(settings, monitor.MonitorSetting{
		Name:      "first test handler",
		Directory: firstDir,
		EventHandler: func(e fsnotify.Event, s string, log *slog.Logger) {
			t.Errorf("This event handler should not have been triggered")
		},
	})

	settings = append(settings, monitor.MonitorSetting{
		Name:      "second test handler",
		Directory: secondDir,
		EventHandler: func(e fsnotify.Event, s string, log *slog.Logger) {
			resultChannel <- result{
				true,
				fsnotify.Create,
			}
		},
	})

	monitorSetup := monitor.Monitor{
		Logger:   log,
		Settings: settings,
	}

	// Need to get a signal back for when the monitor has started
	w, _ := monitorSetup.StartMonitoring()
	defer w.Close()

	// Then create a file or delete in the testing directory
	os.Create(fileToCreate)

	outcome := <-resultChannel
	if !outcome.bool || outcome.Op != fsnotify.Create {
		t.Errorf("Expected true create, received %t, %s", outcome.bool, outcome.Op.String())
	}
}
