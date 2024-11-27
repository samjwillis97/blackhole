package arr_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/debrid"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
	"github.com/spf13/viper"
)

func createTestFile(fileType torrents.TorrentType) (string, string) {
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

func createProcessingDir(rootDir string) string {
	processingDir := path.Join(rootDir, "test_processing")
	if _, err := os.Stat(processingDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(processingDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	return processingDir
}

func TestNewTorrentFileCreated(t *testing.T) {
	requestMade := false
	debridapikey := "123456789"
	startTime := time.Now()
	rootDir, createdFile := createTestFile(torrents.TorrentFile)
	sonarrProcessingPath := createProcessingDir(rootDir)
	sonarrCompletedPath := path.Join(rootDir, "completed_test")
	fileWithNoExtension := strings.Split(createdFile, ".")[0]

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", debridapikey) {
			t.Errorf("Expected a correct Authorization header, got %s", r.Header.Get("Authorization"))
		}
		if r.Method != "PUT" {
			t.Errorf("Expected a 'PUT', got %s", r.Method)
		}
		if r.URL.Path != "/torrents/addTorrent" {
			t.Errorf("Expected to request '/torrents/addTorrent', got %s", r.URL.Path)
		}

		// TODO: Check body contents

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockViper := viper.New()
	mockViper.Set("real_debrid.url", server.URL)
	mockViper.Set("real_debrid.mount_timeout", 10)
	mockViper.Set("sonarr.processing_path", sonarrProcessingPath)
	mockViper.Set("sonarr.completed_path", sonarrCompletedPath)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("debridapikey", debridapikey)
	config.InitializeSecrets(mockSecretViper)

	event := fsnotify.Event{
		Name: createdFile,
		Op:   fsnotify.Create,
	}

	arr.SonarrMonitorHandler(event, rootDir)

	processingFile := path.Join(sonarrProcessingPath, createdFile)
	_, err := os.Stat(processingFile)
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected to find a file in processing at %s, did not.", processingFile)
	}

	if !requestMade {
		t.Errorf("Expected a request to be made, but was not")
	}

	monitoredMeta := debrid.GetMonitoredFile(fileWithNoExtension)
	if monitoredMeta.CompletedDir != sonarrCompletedPath {
		t.Errorf("Expected debrid mount monitor to have completed path %s, got %s", sonarrCompletedPath, monitoredMeta.CompletedDir)
	}
	if monitoredMeta.Expiration.Before(startTime) {
		t.Errorf("Expected debrid mount monitor to have time after %v, got %v", startTime, monitoredMeta.Expiration)
	}
}

func TestNewMagnetFileCreated(t *testing.T) {
	requestMade := false
	debridapikey := "123456789"
	startTime := time.Now()
	rootDir, createdFile := createTestFile(torrents.Magnet)
	sonarrProcessingPath := createProcessingDir(rootDir)
	sonarrCompletedPath := path.Join(rootDir, "completed_test")
	fileWithNoExtension := strings.Split(createdFile, ".")[0]

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", debridapikey) {
			t.Errorf("Expected a correct Authorization header, got %s", r.Header.Get("Authorization"))
		}
		if r.Method != "POST" {
			t.Errorf("Expected a 'POST', got %s", r.Method)
		}
		if r.URL.Path != "/torrents/addMagnet" {
			t.Errorf("Expected to request '/torrents/addMagnet', got %s", r.URL.Path)
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		bodyString := string(bodyBytes)
		if bodyString != `{"magnet":"Test Data"}` {
			t.Errorf("Body was incorrect, got %s", bodyString)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockViper := viper.New()
	mockViper.Set("real_debrid.url", server.URL)
	mockViper.Set("real_debrid.mount_timeout", 10)
	mockViper.Set("sonarr.processing_path", sonarrProcessingPath)
	mockViper.Set("sonarr.completed_path", sonarrCompletedPath)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("debridapikey", debridapikey)
	config.InitializeSecrets(mockSecretViper)

	event := fsnotify.Event{
		Name: createdFile,
		Op:   fsnotify.Create,
	}

	arr.SonarrMonitorHandler(event, rootDir)

	processingFile := path.Join(sonarrProcessingPath, createdFile)
	_, err := os.Stat(processingFile)
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected to find a file in processing at %s, did not.", processingFile)
	}

	if !requestMade {
		t.Errorf("Expected a request to be made, but was not")
	}

	monitoredMeta := debrid.GetMonitoredFile(fileWithNoExtension)
	if monitoredMeta.CompletedDir != sonarrCompletedPath {
		t.Errorf("Expected debrid mount monitor to have completed path %s, got %s", sonarrCompletedPath, monitoredMeta.CompletedDir)
	}
	if monitoredMeta.Expiration.Before(startTime) {
		t.Errorf("Expected debrid mount monitor to have time after %v, got %v", startTime, monitoredMeta.Expiration)
	}
}
