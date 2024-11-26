package arr_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
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

func TestNewTorrentFileCreated(t *testing.T) {
	requestMade := false
	rootDir, createdFile := createTestFile(torrents.TorrentFile)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		if r.Header.Get("Authorization") != "Bearer 123456789" {
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
		// w.Write([]byte(`{"value":"fixed"}`))
	}))
	defer server.Close()

	mockViper := viper.New()
	mockViper.Set("sonarr.url", server.URL)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("debridapikey", "123456789")
	config.InitializeSecrets(mockSecretViper)

	event := fsnotify.Event{
		Name: createdFile,
		Op:   fsnotify.Create,
	}

	arr.SonarrHandler(event, rootDir)

	if !requestMade {
		t.Errorf("Expected a request to be made, but was not")
	}
}

func TestNewMagnetFileCreated(t *testing.T) {
	requestMade := false
	rootDir, createdFile := createTestFile(torrents.Magnet)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		if r.Header.Get("Authorization") != "Bearer 123456789" {
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
		// w.Write([]byte(`{"value":"fixed"}`))
	}))
	defer server.Close()

	mockViper := viper.New()
	mockViper.Set("sonarr.url", server.URL)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("debridapikey", "123456789")
	config.InitializeSecrets(mockSecretViper)

	event := fsnotify.Event{
		Name: createdFile,
		Op:   fsnotify.Create,
	}

	arr.SonarrHandler(event, rootDir)

	if !requestMade {
		t.Errorf("Expected a request to be made, but was not")
	}
}
