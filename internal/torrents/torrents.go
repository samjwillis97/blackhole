package torrents

import (
	"errors"
	"log"
	"os"
	"path"
	"strings"
)

type TorrentType int

const (
	Magnet TorrentType = iota
	TorrentFile
)

type ToProcess struct {
	FullPath      string
	Filename      string
	FilenameNoExt string
	FileType      TorrentType
}

// NewFileToProcess looks at the given file path, and moves the
// file into the proccesing directory, creating the directory if
// required. Then returning the new path back as well as the filename
func NewFileToProcess(filePath string, processingLocation string) (ToProcess, error) {
	log.Printf("[torrents]\t\tmoving %s to process\n", filePath)
	_, filename := path.Split(filePath)
	processingPath := path.Join(processingLocation, filename)

	err := os.Rename(filePath, processingPath)
	if err != nil {
		return ToProcess{}, err
	}

	torrentType, err := getTorrentType(filePath)
	if err != nil {
		return ToProcess{}, err
	}

	ext := path.Ext(filename)
	filenameNoExt := strings.TrimSuffix(filename, ext)

	return ToProcess{
		FullPath:      processingPath,
		Filename:      filename,
		FilenameNoExt: filenameNoExt,
		FileType:      torrentType,
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
