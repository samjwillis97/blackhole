package torrents

import (
	"errors"
	"log"
	"os"
	"path"
)

type TorrentType int

const (
	Magnet TorrentType = iota
	TorrentFile
)

type ToProcess struct {
	FullPath string
	Filename string
	FileType TorrentType
}

const PROCESSING_FOLDER = "processing"

// NewFileToProcess looks at the given file path, and moves the
// file into the proccesing directory, creating the directory if
// required. Then returning the new path back as well as the filename
func NewFileToProcess(filePath string) (ToProcess, error) {
	log.Printf("Starting to process %s\n", filePath)
	rootDir, filename := path.Split(filePath)
	processingPath := path.Join(rootDir, PROCESSING_FOLDER, filename)

	err := checkAndCreateProcessingDir(rootDir)
	if err != nil {
		return ToProcess{}, err
	}

	err = os.Rename(filePath, processingPath)
	if err != nil {
		return ToProcess{}, err
	}

	torrentType, err := getTorrentType(filePath)
	if err != nil {
		return ToProcess{}, err
	}

	return ToProcess{
		FullPath: processingPath,
		Filename: filename,
		FileType: torrentType,
	}, nil
}

func getTorrentType(filename string) (TorrentType, error) {
	if path.Ext(filename) == ".torrent" {
		return TorrentFile, nil
	}

	if path.Ext(filename) == ".magnet" {
		return Magnet, nil
	}

	return 0, errors.New("Not a valid torrent file")
}

func checkAndCreateProcessingDir(rootdir string) error {
	processingPath := path.Join(rootdir, PROCESSING_FOLDER)
	if _, err := os.Stat(processingPath); errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(processingPath, os.ModePerm)
	}

	return nil
}
