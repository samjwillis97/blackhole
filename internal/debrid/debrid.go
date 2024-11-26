package debrid

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/samjwillis97/sams-blackhole/internal/config"
)

// const BASE_URL = "https://api.real-debrid.com/rest/1.0/"

// funcs like

func blessRequest(r *http.Request) *http.Request {
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.GetSecrets().DebridApiKey))

	return r
}

// Contents of a magnet file contain the magnet link
func AddMagnet(filepath string) {
	url, err := url.Parse(config.GetAppConfig().Sonarr.Url)
	url = url.JoinPath("torrents/addMagnet")

	// There might be a better way of getting bytes into buffer
	data, err := os.ReadFile(filepath)
	if err != nil {
		panic(err)
	}

	payload := []byte(fmt.Sprintf(`{"magnet":"%s"}`, data))

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}

	req = blessRequest(req)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		panic(errors.New("Unable to make request"))
	}

	fmt.Println(resp.StatusCode)
}

func AddTorrent(filepath string) {
	url, err := url.Parse(config.GetAppConfig().Sonarr.Url)
	url = url.JoinPath("torrents/addTorrent")

	// There might be a better way of getting bytes into buffer
	data, err := os.ReadFile(filepath)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}

	req = blessRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		panic(errors.New("Unable to make request"))
	}

	fmt.Println(resp.StatusCode)
}
