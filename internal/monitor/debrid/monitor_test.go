package debrid_test

import (
	"errors"
	"log/slog"
	"os"
	"path"
	"testing"

	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/logger"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/debrid"
	"github.com/spf13/viper"
)

type setupOutput struct {
	ToLinkDir      string
	ProcessingFile string
	InternalFiles  []string
	CompletedDir   string
}

func setup() setupOutput {
	processingFile, err := os.CreateTemp("", "debrid-test-processing-file-*")
	toLinkDir, err := os.MkdirTemp("", "debrid-test-torrent-*")
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

	completedDir, err := os.MkdirTemp("", "debrid-test-completed-*")
	if err != nil {
		panic(err)
	}

	mockViper := viper.New()
	mockViper.Set("real_debrid.url", "http://localhost")
	mockViper.Set("real_debrid.mount_timeout", 30)
	mockViper.Set("real_debrid.watch_path", "/tmp")
	config.InitializeAppConfig(mockViper)

	return setupOutput{ProcessingFile: processingFile.Name(), ToLinkDir: toLinkDir, CompletedDir: completedDir, InternalFiles: files}
}

func cleanup(setup setupOutput) {
	os.Remove(setup.ProcessingFile)
	os.RemoveAll(setup.ToLinkDir)
	os.RemoveAll(setup.CompletedDir)
}

// TODO: Given When Then
func TestMountMonitorDirCreated(t *testing.T) {
	log := slog.New(logger.NewHandler(&slog.HandlerOptions{Level: slog.LevelDebug}))

	setupConfig := setup()
	defer cleanup(setupConfig)

	log.Info("Setup", "data", setupConfig)

	debrid.MonitorForDebridFiles(debrid.MonitorConfig{
		Filename:         path.Base(setupConfig.ToLinkDir),
		OriginalFilename: path.Base(setupConfig.ToLinkDir),
		CompletedDir:     setupConfig.CompletedDir,
		ProcessingPath:   setupConfig.ProcessingFile,
		Service:          arr.Sonarr,
		Callbacks: debrid.Callbacks{
			Success: func() error { return nil },
			Failure: func() {},
		},
	}, log)

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

	if _, err := os.Lstat(setupConfig.ProcessingFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Processing file still exists at %s", setupConfig.ProcessingFile)
	}
}
