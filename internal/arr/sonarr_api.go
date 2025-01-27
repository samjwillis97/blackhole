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

type SonarrCommandResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SonarrHistoryItemEventType string

const (
	Unknown                SonarrHistoryItemEventType = "unknown"
	Grabbed                                           = "grabbed"
	SeriesFolderImported                              = "seriesFolderImported"
	DownloadFolderImported                            = "downloadFolderImported"
	DownloadFailed                                    = "downloadFailed"
	EpisodeFileDeleted                                = "episodeFileDeleted"
	EpisodeFileRenamed                                = "episodeFileRenamed"
	DownloadIgnored                                   = "downloadIgnored"
)

type SonarrHistoryResponse struct {
	Page         int                 `json:"page"`
	PageSize     int                 `json:"pageSize"`
	TotalRecords int                 `json:"totalRecords"`
	Records      []SonarrHistoryItem `json:"records"`
}

type SonarrHistoryItem struct {
	ID          int                        `json:"id"`
	SourceTitle string                     `json:"sourceTitle"`
	EventType   SonarrHistoryItemEventType `json:"eventType"`
	Data        SonarrHistoryItemData      `json:"data"`
}

type SonarrHistoryItemData struct {
	TorrentInfoHash string `json:"torrentInfoHash"`
}

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

func (s *SonarrClient) RefreshMonitoredDownloads() (SonarrCommandResponse, error) {
	url := s.URL.JoinPath("/api/v3/command")

	payload := []byte(`{"name":"RefreshMonitoredDownloads"}`)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(payload))
	if err != nil {
		return SonarrCommandResponse{}, err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return SonarrCommandResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return SonarrCommandResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse SonarrCommandResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return SonarrCommandResponse{}, err
	}

	return apiResponse, nil
}

func (s *SonarrClient) SonarrGetHistory(pagesize int) (SonarrHistoryResponse, error) {
	url := s.URL.JoinPath("/api/v3/history")

	query := url.Query()
	query.Add("pageSize", fmt.Sprintf("%d", pagesize))
	url.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return SonarrHistoryResponse{}, err
	}

	req = s.blessSonarrRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return SonarrHistoryResponse{}, err
	}

	if resp.StatusCode != 200 {
		// TODO: Trace log
		return SonarrHistoryResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse SonarrHistoryResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return SonarrHistoryResponse{}, err
	}

	return apiResponse, nil
}

// Have to get the ID from the history endpoint, will investigate what the mapping is
func (s *SonarrClient) SonarrFailHistoryItem(id int) error {
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
