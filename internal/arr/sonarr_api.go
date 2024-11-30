package arr

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"

	"github.com/samjwillis97/sams-blackhole/internal/config"
)

// TODO: implement retries

func blessSonarrRequest(r *http.Request) *http.Request {
	r.Header.Set("X-Api-Key", config.GetSecrets().SonarrApiKey)
	r.Header.Set("Content-Type", "application/json")

	return r
}

func SonarrRefreshMonitoredDownloads() {
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
}
