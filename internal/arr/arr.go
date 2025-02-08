package arr

type ArrService int

const (
	Sonarr ArrService = iota
	Radarr
)

func (s ArrService) String() string {
	switch s {
	case Sonarr:
		return "Sonarr"
	case Radarr:
		return "Radarr"
	}

	return "Unknown"
}

type HistoryItemEventType string

const (
	Unknown                HistoryItemEventType = "unknown"
	Grabbed                                     = "grabbed"
	MovieFolderImported                         = "movieFolderImported"
	SeriesFolderImported                        = "seriesFolderImported"
	DownloadFolderImported                      = "downloadFolderImported"
	DownloadFailed                              = "downloadFailed"
	MovieFileDeleted                            = "movieFileDeleted"
	EpisodeFileDeleted                          = "episodeFileDeleted"
	MovieFileRename                             = "movieFileRename"
	EpisodeFileRenamed                          = "episodeFileRenamed"
	DownloadIgnored                             = "downloadIgnored"
)

type HistoryItemData struct {
	TorrentInfoHash string `json:"torrentInfoHash"`
}

type HistoryItem struct {
	ID          int                  `json:"id"`
	SourceTitle string               `json:"sourceTitle"`
	EventType   HistoryItemEventType `json:"eventType"`
	Data        HistoryItemData      `json:"data"`
}

type HistoryResponse struct {
	Page         int           `json:"page"`
	PageSize     int           `json:"pageSize"`
	TotalRecords int           `json:"totalRecords"`
	Records      []HistoryItem `json:"records"`
}

type CommandResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ArrClient interface {
	FailHistoryItem(id int) error
	GetHistory(pagesize int) (HistoryResponse, error)
	RefreshMonitoredDownloads() (CommandResponse, error)
}
