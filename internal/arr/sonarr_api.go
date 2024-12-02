package arr

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/samjwillis97/sams-blackhole/internal/config"
)

type SonarrCommandResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TODO: implement retries
// TODO: Maybe just a request wrapper for logging as well

func blessSonarrRequest(r *http.Request) *http.Request {
	r.Header.Set("X-Api-Key", config.GetSecrets().SonarrApiKey)
	r.Header.Set("Content-Type", "application/json")

	return r
}

func SonarrRefreshMonitoredDownloads() SonarrCommandResponse {
	url, err := url.Parse(config.GetAppConfig().Sonarr.Url)
	url = url.JoinPath("/api/v3/command")

	payload := []byte(`{"name":"RefreshMonitoredDownloads"}`)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}

	req = blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		panic(errors.New("Unable to make request"))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse SonarrCommandResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		panic(err)
	}

	return apiResponse
}
