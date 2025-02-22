package torrents

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/jackpal/bencode-go"
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

	magnet string
}

type infoDict struct {
	Pieces      string     `bencode:"pieces"`
	PieceLength int        `bencode:"piece length"`
	Name        string     `bencode:"name,omitempty"`
	NameUTF8    string     `bencode:"name.utf-8,omitempty"`
	Files       []fileDict `bencode:"files,omitempty"`
	Length      int64      `bencode:"length,omitempty"`
	Private     int        `bencode:"private,omitempty"`
	MD5Sum      string     `bencode:"md5sum,omitempty"`
}

type fileDict struct {
	Length   int64    `bencode:"length"`
	Path     []string `bencode:"path"`
	PathUTF8 []string `bencode:"path.utf-8,omitempty"`
}

type torrentFile struct {
	Info infoDict `bencode:"info"`
}

// NewFileToProcess looks at the given file path, and moves the
// file into the proccesing directory, creating the directory if
// required. Then returning the new path back as well as the filename
func NewFileToProcess(filePath string, processingLocation string) (ToProcess, error) {
	_, filename := path.Split(filePath)
	processingPath := path.Join(processingLocation, filename)
	ext := path.Ext(filename)
	filenameNoExt := strings.TrimSuffix(filename, ext)

	err := os.Rename(filePath, processingPath)
	if err != nil {
		return ToProcess{}, err
	}

	torrentType, err := getTorrentType(filePath)
	if err != nil {
		return ToProcess{}, err
	}

	toProcess := ToProcess{
		FullPath:      processingPath,
		Filename:      filename,
		FilenameNoExt: filenameNoExt,
		FileType:      torrentType,
	}

	// TODO: Here I want to open the magnet file and store in struct
	// Then create a method to `getMagnet` that would return it.
	// This would throw an error if the torrent is not a magnet file

	return toProcess, nil
}

func (t *ToProcess) GetMagnetLink() (string, error) {
	if t.FileType != Magnet {
		return "", errors.New("Unable to get magnet for torrent file")
	}

	fileContent, err := os.ReadFile(t.FullPath)
	if err != nil {
		return "", err
	}
	return string(fileContent), nil
}

func (t *ToProcess) GetHash() (string, error) {
	fileContent, err := os.ReadFile(t.FullPath)
	if err != nil {
		return "", err
	}

	switch t.FileType {
	case TorrentFile:
		return getTorrentFileInfoHash(fileContent)
	case Magnet:
		return getMagnetLinkInfoHash(string(fileContent))
	}

	return "", errors.New("Unknown file type")
}

func FromFileInProcessing(filePath string) (ToProcess, error) {
	_, filename := path.Split(filePath)

	torrentType, err := getTorrentType(filePath)
	if err != nil {
		return ToProcess{}, err
	}

	ext := path.Ext(filename)
	filenameNoExt := strings.TrimSuffix(filename, ext)

	return ToProcess{
		FullPath:      filePath,
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

func getTorrentFileInfoHash(file []byte) (string, error) {
	torrent := torrentFile{}
	fileReader := bytes.NewReader(file)
	err := bencode.Unmarshal(fileReader, &torrent)
	if err != nil {
		return "", err
	}

	var infoBuffer bytes.Buffer
	err = bencode.Marshal(&infoBuffer, torrent.Info)
	if err != nil {
		return "", err
	}

	hash := sha1.Sum(infoBuffer.Bytes())
	infoHash := hex.EncodeToString(hash[:])

	return infoHash, nil
}

// See: https://github.com/alanmcgovern/monotorrent/blob/9e98a44c3af93ace7fe11da363fe345a60c0c93f/src/MonoTorrent.Client/MonoTorrent/MagnetLink.cs#L120
func getMagnetLinkInfoHash(magnetLink string) (string, error) {
	// Link starts with `magnet:?`
	// After the ? is the parmaters seperated by `&`
	if !strings.HasPrefix(magnetLink, "magnet:?") {
		return "", errors.New("Does not appear to be a valid magnet")
	}

	// This could be wrong
	splitLink := strings.SplitAfter(magnetLink, "magnet:?")
	if len(splitLink) != 2 {
		return "", errors.New("Unable to split off parameters from magnet")
	}

	parameters := strings.Split(splitLink[1], "&")
	for _, v := range parameters {
		keyValSlice := strings.Split(v, "=")
		key := keyValSlice[0]

		if key != "xt" {
			continue
		}

		val := keyValSlice[1]
		topicIdentifier := val[0:9] // Need to check whether this is inclusive or not
		topicValue := val[9:]

		switch topicIdentifier {
		case "urn:sha1:":
		case "urn:btih:":
			return topicValue, nil
			// if len(topicValue) == 32 {
			// 	// FromBase32
			// }
			// if len(topicValue) == 40 {
			// 	// FromHex
			// }
			// return "", errors.New(fmt.Sprintf("Unsure how to handle hash of length %d", len(topicValue)))
		default:
			return "", errors.New(fmt.Sprintf("Unable to extract hash from %s", topicIdentifier))
		}
	}

	return "", errors.New("Unable to get Hash")
}
