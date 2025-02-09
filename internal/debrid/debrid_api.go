package debrid

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"github.com/samjwillis97/sams-blackhole/internal/config"
)

type AddTorrentResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

type DebridStatus string

const (
	Downloaded           DebridStatus = "downloaded"
	MagnetError                       = "magnet_error"
	MagnetConversion                  = "magnet_conversion"
	WaitingFileSelection              = "waiting_files_selection"
	Queued                            = "queued"
	Downloading                       = "downloading"
	Error                             = "error"
	Virus                             = "virus"
	Compressing                       = "compressing"
	Uploading                         = "uploading"
	Dead                              = "dead"
)

type GetInfoResponse struct {
	Filename         string       `json:"filename"`
	OriginalFilename string       `json:"original_filename"`
	Status           DebridStatus `json:"status"`
}

func blessRequest(r *http.Request) *http.Request {
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.GetSecrets().GetString("DEBRID_API_KEY")))

	return r
}

// TODO: implement retries
// TODO: Maybe put a lock around this to ensure not too many requests at once
// And they can all share the same retry mechanism to not overload

// Contents of a magnet file contain the magnet link
func AddMagnet(magnetLink string) (AddTorrentResponse, error) {
	reqUrl, err := url.Parse(config.GetAppConfig().RealDebrid.Url)
	reqUrl = reqUrl.JoinPath("torrents/addMagnet")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	err = writer.WriteField("magnet", magnetLink)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	req, err := http.NewRequest(http.MethodPost, reqUrl.String(), &body)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	req = blessRequest(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return AddTorrentResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d, message: %s", resp.StatusCode, string(bodyBytes)))
	}

	var apiResponse AddTorrentResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	return apiResponse, nil
}

func SelectFiles(torrentId string, fileIds []string) error {
	reqUrl, err := url.Parse(config.GetAppConfig().RealDebrid.Url)
	if err != nil {
		return err
	}
	reqUrl = reqUrl.JoinPath(fmt.Sprintf("torrents/selectFiles/%s", torrentId))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	filesToSelect := "all"
	if len(fileIds) > 0 {
		filesToSelect = ""
		for i, id := range fileIds {
			if i > 0 {
				filesToSelect += ","
			}
			filesToSelect += id
		}
	}

	err = writer.WriteField("files", filesToSelect)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, reqUrl.String(), &body)
	if err != nil {
		return err
	}

	req = blessRequest(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	// bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	return nil
}

func GetInfo(torrentId string) (GetInfoResponse, error) {
	url, err := url.Parse(config.GetAppConfig().RealDebrid.Url)
	if err != nil {
		return GetInfoResponse{}, err
	}
	url = url.JoinPath(fmt.Sprintf("torrents/info/%s", torrentId))

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return GetInfoResponse{}, err
	}

	req = blessRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return GetInfoResponse{}, err
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return GetInfoResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	var apiResponse GetInfoResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return GetInfoResponse{}, err
	}

	return apiResponse, nil
}

func AddTorrent(filepath string) (AddTorrentResponse, error) {
	url, err := url.Parse(config.GetAppConfig().RealDebrid.Url)
	if err != nil {
		return AddTorrentResponse{}, err
	}
	url = url.JoinPath("torrents/addTorrent")

	// There might be a better way of getting bytes into buffer
	data, err := os.ReadFile(filepath)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return AddTorrentResponse{}, err
	}

	req = blessRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// TODO: Trace log
		return AddTorrentResponse{}, errors.New(fmt.Sprintf("Unable to make request response code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse AddTorrentResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		return AddTorrentResponse{}, err
	}

	return apiResponse, nil
}

func Remove(id string) error {
	url, err := url.Parse(config.GetAppConfig().RealDebrid.Url)
	if err != nil {
		return err
	}
	url = url.JoinPath(fmt.Sprintf("torrents/delete/%s", id))

	req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return err
	}

	req = blessRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		// TODO: Trace log
		return errors.New(fmt.Sprintf("Failed to delete with status code: %d", resp.StatusCode))
	}

	return nil
}
