package debrid

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
)

type MonitorConfig struct {
	Filename         string
	OriginalFilename string
	CompletedDir     string
	ProcessingPath   string
	Service          arr.ArrService
	Callbacks        Callbacks
}

func MonitorForDebridFiles(c MonitorConfig, logger *slog.Logger) {
	expectedPath := path.Join(config.GetAppConfig().RealDebrid.WatchPatch, c.Filename)

	timeout := time.Duration(config.GetAppConfig().RealDebrid.MountTimeout) * time.Second
	expiry := time.Now().Add(timeout)

	logger.Info("adding path to debrid watch list", "expiry", expiry)

	pathSet := getPathSetInstance()
	meta := PathMeta{
		Expiration:       expiry,
		OriginalFileName: c.OriginalFilename,
		Service:          c.Service,
		CompletedDir:     c.CompletedDir,
		ProcessingPath:   c.ProcessingPath,
		Callbacks:        c.Callbacks,
	}
	pathSet.add(c.Filename, meta)

	if _, err := os.Stat(expectedPath); err == nil {
		logger.Info("path already exists in debrid mount, going to process")
		newMountFileOrDir(expectedPath, c.Filename, logger)
		return
	}
}

func MonitorHandler(e watcher.Event, _ string, logger *slog.Logger) {
	name := path.Base(e.Path)

	switch e.Op {
	case watcher.Create:
		monitor.Debounce(e.Path, monitor.CreateOrWrite, func() {
			newMountFileOrDir(e.Path, name, logger)
		})
	}
}

func newMountFileOrDir(newPath string, name string, logger *slog.Logger) {
	pathSet := getPathSetInstance()

	// FIXME: Eventually also check the originalfilename, I guess
	pathMeta, err := pathSet.remove(name)

	if err != nil {
		logger.Debug("not monitoring for, skipping")
		return
	}

	logger = logger.With("outputDir", pathMeta.CompletedDir)
	logger = logger.With("processingPath", pathMeta.ProcessingPath)

	if _, err := os.Stat(pathMeta.ProcessingPath); err != nil {
		logger.Warn("doesn't exist anymore, not processing")
		return
	}

	logger.Info("starting symlinking")
	completedPath := path.Join(pathMeta.CompletedDir, name)
	err = os.Mkdir(completedPath, os.ModePerm)
	if err != nil {
		logger.Error("failed to link", "err", err)
		return
	}

	logger.Debug("recursively symlinking")

	err = filepath.WalkDir(newPath, func(currentFile string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(newPath, currentFile)
		if err != nil {
			return err
		}

		currentFileToMove := path.Join(completedPath, relativePath)

		if strings.Contains(relativePath, "../") {
			return errors.New("File appears to be from outside root dir")
		}

		toMoveParentDir, _ := path.Split(currentFileToMove)
		err = os.MkdirAll(toMoveParentDir, os.ModePerm)
		if err != nil {
			return err
		}

		err = os.Symlink(currentFile, currentFileToMove)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		logger.Error("recursive linking failed", "err", err)
		return
	}

	logger.Info("symlinking complete")

	err = pathMeta.Callbacks.Success()
	if err != nil {
		logger.Warn("success callback failed", "err", err)
		pathMeta.Callbacks.Failure()
	}

	logger.Debug("removing from processing")
	err = os.Remove(pathMeta.ProcessingPath)
	if err != nil {
		logger.Error("failed to delete processing file", "err", err)
		return
	}

}
