package debrid_test

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/debrid"
	"github.com/spf13/viper"
)

type setupOutput struct {
	ToLinkDir     string
	InternalFiles []string
	CompletedDir  string
}

func setup() setupOutput {
	toLinkDir, err := os.MkdirTemp("", "debrid-monitor-test-torrent-*")
	if err != nil {
		panic(err)
	}

	files := []string{"root-file", "subfolder/file"}
	for _, f := range files {
		pathToCreate := path.Join(toLinkDir, f)
		rootDir, _ := path.Split(pathToCreate)

		err := os.MkdirAll(rootDir, os.ModePerm)
		if err != nil {
			panic(err)
		}

		_, err = os.Create(pathToCreate)
		if err != nil {
			panic(err)
		}
	}

	completedDir, err := os.MkdirTemp("", "debrid-monitor-test-completed-*")
	if err != nil {
		panic(err)
	}

	mockViper := viper.New()
	mockViper.Set("real_debrid.url", "http://localhost")
	mockViper.Set("real_debrid.mount_timeout", 30)
	config.InitializeAppConfig(mockViper)

	return setupOutput{ToLinkDir: toLinkDir, CompletedDir: completedDir, InternalFiles: files}
}

func cleanup(setup setupOutput) {
	os.RemoveAll(setup.ToLinkDir)
	os.RemoveAll(setup.CompletedDir)
}

func TestMountMonitorDirCreated(t *testing.T) {
	setupConfig := setup()

	debrid.MonitorForFiles(path.Base(setupConfig.ToLinkDir), setupConfig.CompletedDir)

	event := fsnotify.Event{
		Name: path.Base(setupConfig.ToLinkDir),
		Op:   fsnotify.Create,
	}

	rootDir, _ := path.Split(setupConfig.ToLinkDir)
	debrid.DebridMountMonitorHandler(event, rootDir)

	expectedOutputDir := path.Join(setupConfig.CompletedDir, path.Base(setupConfig.ToLinkDir))
	if _, err := os.Stat(expectedOutputDir); errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected a directory at %s", expectedOutputDir)
	}

	for _, f := range setupConfig.InternalFiles {
		expectedSymlinkedPath := path.Join(expectedOutputDir, f)
		if _, err := os.Lstat(expectedSymlinkedPath); errors.Is(err, os.ErrNotExist) {
			t.Errorf("Expected a symlink at %s", expectedSymlinkedPath)
		}
	}

	cleanup(setupConfig)
}
