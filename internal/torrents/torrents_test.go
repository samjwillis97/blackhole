package torrents_test

import (
	"strings"
	"testing"

	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

func TestGettingTorrentFileInfoHash(t *testing.T) {
	filePath := "./testfiles/test.torrent"

	torrent, err := torrents.NewFileToProcess(filePath, "./testfiles")
	if err != nil {
		t.Errorf("Error occurred: %s", err)
	}

	hash, err := torrent.GetHash()
	if err != nil {
		t.Errorf("Error occurred: %s", err)
	}

	expected := "150947B245DA89629349290C2812ECDB6D0308C7"
	if strings.ToUpper(hash) != expected {
		t.Errorf("Expected hash to be %s, received %s", expected, strings.ToUpper(hash))
	}
}

func TestGettingMagnetInfoHash(t *testing.T) {
	filePath := "./testfiles/test.magnet"

	torrent, err := torrents.NewFileToProcess(filePath, "./testfiles")
	if err != nil {
		t.Errorf("Error occurred: %s", err)
	}

	hash, err := torrent.GetHash()
	if err != nil {
		t.Errorf("Error occurred: %s", err)
	}

	expected := "150947B245DA89629349290C2812ECDB6D0308C7"
	if strings.ToUpper(hash) != expected {
		t.Errorf("Expected hash to be %s, received %s", expected, strings.ToUpper(hash))
	}
}
