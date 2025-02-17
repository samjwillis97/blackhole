package arr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SonarrClient struct {
	URL    *url.URL
	APIKey string
}

func CreateNewSonarrClient(baseUrl string, apiKey string) (*SonarrClient, error) {
	url, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	return &SonarrClient{
		URL:    url,
		APIKey: apiKey,
	}, nil
}

// TODO: implement retries
// TODO: Maybe just a request wrapper for logging as well

func (s *SonarrClient) blessSonarrRequest(r *http.Request) *http.Request {
	r.Header.Set("X-Api-Key", s.APIKey)
	r.Header.Set("Content-Type", "application/json")

	return r
}

func (s *SonarrClient) RefreshMonitoredDownloads() (CommandResponse, error) {
	url := s.URL.JoinPath("/api/v3/command")

	payload := []byte(`{"name":"RefreshMonitoredDownloads"}`)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(payload))
	if err != nil {
		return CommandResponse{}, err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return CommandResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return CommandResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse CommandResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return CommandResponse{}, err
	}

	return apiResponse, nil
}

func (s *SonarrClient) GetHistory(pagesize int) (HistoryResponse, error) {
	url := s.URL.JoinPath("/api/v3/history")

	query := url.Query()
	query.Add("pageSize", fmt.Sprintf("%d", pagesize))
	query.Add("includeEpisode", "true")
	url.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return HistoryResponse{}, err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return HistoryResponse{}, err
	}

	if resp.StatusCode != 200 {
		// TODO: Trace log
		return HistoryResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse HistoryResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return HistoryResponse{}, err
	}

	return apiResponse, nil
}

// Have to get the ID from the history endpoint, will investigate what the mapping is
func (s *SonarrClient) FailHistoryItem(id int) error {
	url := s.URL.JoinPath("/api/v3/history/failed", fmt.Sprintf("%d", id))

	req, err := http.NewRequest(http.MethodPost, url.String(), nil)
	if err != nil {
		return err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		// TODO: Trace log
		return errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	return nil
}

func (s *SonarrClient) SearchSeason(seriesId int, seasonNumber int) (CommandResponse, error) {
	url := s.URL.JoinPath("/api/v3/command")

	payload := []byte(
		fmt.Sprintf(`{"name":"SeasonSearch", "seriesId": %d, "seasonNumber": %d}`, seriesId, seasonNumber),
	)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(payload))
	if err != nil {
		return CommandResponse{}, err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return CommandResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return CommandResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse CommandResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return CommandResponse{}, err
	}

	return apiResponse, nil
}
