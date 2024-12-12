package sonarr_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
	"github.com/samjwillis97/sams-blackhole/internal/monitor/sonarr"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
	"github.com/spf13/viper"
)

func createTestFile2(fileType torrents.TorrentType) (string, string) {
	var fileName string
	tempDir := os.TempDir()

	switch fileType {
	case torrents.TorrentFile:
		fileName = "file.torrent"
	case torrents.Magnet:
		fileName = "file.magnet"
	}

	fileToCreate := path.Join(tempDir, fileName)
	_, err := os.Stat(fileToCreate)
	if err != nil {
		os.Remove(fileToCreate)
	}
	os.Create(fileToCreate)
	os.WriteFile(fileToCreate, []byte("Test Data"), os.ModePerm)

	return tempDir, fileName
}

func createProcessingDir2(rootDir string) string {
	processingDir := path.Join(rootDir, "test_processing")
	if _, err := os.Stat(processingDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(processingDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	return processingDir
}

func TestNewMagnetFileCreated2(t *testing.T) {
	requestMade := false
	debridapikey := "123456789"
	startTime := time.Now()
	rootDir, createdFile := createTestFile2(torrents.Magnet)
	sonarrProcessingPath := createProcessingDir2(rootDir)
	sonarrCompletedPath := path.Join(rootDir, "completed_test")

	hasMadeFirstInfoRequest := false

	debridId := "123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		switch r.URL.Path {
		case "/torrents/addMagnet":
			if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", debridapikey) {
				t.Errorf("Expected a correct Authorization header, got %s", r.Header.Get("Authorization"))
			}
			if r.Method != "POST" {
				t.Errorf("Expected a 'POST', got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{
        "id": "%s",
        "uri": "idk-auri"
      }`, debridId)))
		case fmt.Sprintf("/torrents/info/%s", debridId):
			if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", debridapikey) {
				t.Errorf("Expected a correct Authorization header, got %s", r.Header.Get("Authorization"))
			}
			if r.Method != "GET" {
				t.Errorf("Expected a 'GET', got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			status := "queued"
			if hasMadeFirstInfoRequest {
				status = "downloaded"
			}
			w.Write([]byte(fmt.Sprintf(`{
        "filename": "%s",
        "status": "%s"
      }`, createdFile, status)))
			hasMadeFirstInfoRequest = true
		default:
			t.Errorf("Unexpected request to '%s'", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	mockViper := viper.New()
	mockViper.Set("real_debrid.url", server.URL)
	mockViper.Set("real_debrid.mount_timeout", 10)
	mockViper.Set("sonarr.processing_path", sonarrProcessingPath)
	mockViper.Set("sonarr.completed_path", sonarrCompletedPath)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("DEBRID_API_KEY", debridapikey)
	config.InitializeSecrets(mockSecretViper)

	err := sonarr.NewTorrentFile(path.Join(rootDir, createdFile))

	processingFile := path.Join(sonarrProcessingPath, createdFile)
	_, err = os.Stat(processingFile)
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected to find a file in processing at %s, did not.", processingFile)
	}

	if !requestMade {
		t.Errorf("Expected a request to be made, but was not")
	}

	monitoredMeta := monitor.GetMonitoredFile(createdFile)
	if monitoredMeta.CompletedDir != sonarrCompletedPath {
		t.Errorf("Expected debrid mount monitor to have completed path %s, got %s", sonarrCompletedPath, monitoredMeta.CompletedDir)
	}
	if monitoredMeta.Expiration.Before(startTime) {
		t.Errorf("Expected debrid mount monitor to have time after %v, got %v", startTime, monitoredMeta.Expiration)
	}
}
