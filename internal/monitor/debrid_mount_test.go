package monitor_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/monitor"
	"github.com/spf13/viper"
)

type setupInput struct {
	ServerUrl string
	ApiKey    string
}

type setupOutput struct {
	ToLinkDir     string
	InternalFiles []string
	CompletedDir  string
}

func setup(input setupInput) setupOutput {
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
	mockViper.Set("sonarr.url", input.ServerUrl)
	config.InitializeAppConfig(mockViper)

	mockSecretViper := viper.New()
	mockSecretViper.Set("sonarrapikey", input.ApiKey)
	config.InitializeSecrets(mockSecretViper)

	return setupOutput{ToLinkDir: toLinkDir, CompletedDir: completedDir, InternalFiles: files}
}

func cleanup(setup setupOutput) {
	os.RemoveAll(setup.ToLinkDir)
	os.RemoveAll(setup.CompletedDir)
}

// TODO: Given When Then
func TestMountMonitorDirCreated(t *testing.T) {
	apiKey := "test-123"
	requestMade := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		if r.Header.Get("X-Api-Key") != apiKey {
			t.Errorf("Expected a correct X-Api-Key header, got %s", r.Header.Get("X-Api-Key"))
		}
		if r.Method != "POST" {
			t.Errorf("Expected a 'POST', got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/command" {
			t.Errorf("Expected to request '/api/v3/command', got %s", r.URL.Path)
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		bodyString := string(bodyBytes)
		if bodyString != `{"name":"RefreshMonitoredDownloads"}` {
			t.Errorf("Body was incorrect, got %s", bodyString)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	setupConfig := setup(setupInput{
		ServerUrl: server.URL,
		ApiKey:    apiKey,
	})
	defer cleanup(setupConfig)

	monitor.MonitorForDebridFiles(path.Base(setupConfig.ToLinkDir), setupConfig.CompletedDir, arr.Sonarr)

	event := fsnotify.Event{
		Name: path.Base(setupConfig.ToLinkDir),
		Op:   fsnotify.Create,
	}

	rootDir, _ := path.Split(setupConfig.ToLinkDir)
	monitor.DebridMountMonitorHandler(event, rootDir)

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

	if !requestMade {
		t.Errorf("No request was successfully made")
	}
}
