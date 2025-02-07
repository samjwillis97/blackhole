package sonarr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"time"

	"github.com/looplab/fsm"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"github.com/samjwillis97/sams-blackhole/internal/config"
	"github.com/samjwillis97/sams-blackhole/internal/debrid"
	debridMonitor "github.com/samjwillis97/sams-blackhole/internal/monitor/debrid"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

var StateRequiredFields = map[string][]string{
	"processing":       {"IngestedPath"},
	"addingToDebrid":   {"ProcessingTorrent"},
	"debridProcessing": {"debridID"},
}

// Still think there must be a better way of handling data between states
type MonitorItem struct {
	ingestedPath      string
	processingTorrent torrents.ToProcess

	debridID    string
	timeoutTime time.Time
	prettyName  string

	sonarrClient *arr.SonarrClient
	logger       *slog.Logger
	config       config.SonarrConfig

	sm *fsm.FSM
}

func (m *MonitorItem) setProcessingTorrent(t torrents.ToProcess) {
	m.processingTorrent = t
	m.logger = m.logger.With("processingPath", t.FullPath)
}

func (m *MonitorItem) setDebridID(id string) {
	m.debridID = id
	m.logger = m.logger.With("debridID", id)
}

func new(conf config.SonarrConfig, logger *slog.Logger) (*MonitorItem, error) {
	sonarrClient, err := arr.CreateNewSonarrClient(
		conf.Url,
		config.GetSecrets().GetString(fmt.Sprintf("%s_API_KEY", strings.ToUpper(conf.Name))),
	)

	if err != nil {
		return nil, err
	}

	s := &MonitorItem{
		sonarrClient: sonarrClient,
		config:       conf,
		logger:       logger,
	}

	callbacks := fsm.Callbacks{
		"before_event": s.enterState,

		"processing":     s.enterProcessing,
		"addingToDebrid": s.enterAddingToDebrid,

		"debridProcessing": s.enterDebridProcessing,

		"awaitingDebridRetry": s.waitToRetryDebridProcessing,

		"failure":   s.enterFailure,
		"completed": s.enterCompleted,
	}

	events := fsm.Events{
		{Name: "torrentFound", Src: []string{"new"}, Dst: "processing"},
		{Name: "addToDebrid", Src: []string{"new", "processing"}, Dst: "addingToDebrid"},
		{Name: "checkDebridState", Src: []string{"addingToDebrid", "awaitingDebridRetry"}, Dst: "debridProcessing"},
		{Name: "retryDebridProcessing", Src: []string{"debridProcessing"}, Dst: "awaitingDebridRetry"},
		{Name: "complete", Src: []string{"failure", "debridProcessing"}, Dst: "completed"},
	}

	// Adding failure event available for transition from any state
	failureEvent := fsm.EventDesc{Name: "failed", Src: []string{}, Dst: "failure"}
	for k := range callbacks {
		failureEvent.Src = append(failureEvent.Src, k)
	}
	events = append(events, failureEvent)

	s.sm = fsm.NewFSM(
		"new",
		events,
		callbacks,
	)

	return s, nil
}

func NewTorrentFile(conf config.SonarrConfig, filepath string, logger *slog.Logger) error {
	torrentItem, err := new(conf, logger)
	if err != nil {
		return err
	}

	torrentItem.ingestedPath = filepath

	if err := torrentItem.sm.Event(context.Background(), "torrentFound"); err != nil {
		return err
	}

	return nil
}

// TODO: Accept logger here
func ResumeProcessingFile(conf config.SonarrConfig, filepath string, logger *slog.Logger) error {
	torrentItem, err := new(conf, logger)
	if err != nil {
		return err
	}

	toProcess, err := torrents.NewFileToProcess(filepath, conf.ProcessingPath)
	if err != nil {
		return err
	}

	torrentItem.setProcessingTorrent(toProcess)

	if err := torrentItem.sm.Event(context.Background(), "addToDebrid"); err != nil {
		return err
	}

	return nil
}

func (s *MonitorItem) validateFields(requiredFields ...string) error {
	fieldErrors := []string{}
	for _, field := range requiredFields {
		switch field {
		case "IngestedPath":
			if s.ingestedPath == "" {
				fieldErrors = append(fieldErrors, "IngestedPath is not set")
			}
		case "ProcessingTorrent":
			if (s.processingTorrent == torrents.ToProcess{}) {
				fieldErrors = append(fieldErrors, "ProcessingTorrent is not set")
			}
		case "DebridID":
			if s.debridID == "" {
				fieldErrors = append(fieldErrors, "DebridID is not set")
			}
		}
	}

	if len(fieldErrors) > 0 {
		return errors.New("Validation failed: " + strings.Join(fieldErrors, ", "))
	}
	return nil
}

func (s *MonitorItem) enterState(c context.Context, e *fsm.Event) {
	if s.timeoutTime.IsZero() {
		s.timeoutTime = time.Now().Add(30 * time.Second)
	}

	if time.Now().After(s.timeoutTime) {
		s.sm.Event(c, "failed", errors.New("timed out"))
	}

	s.logger.Debug(fmt.Sprintf("entering %s", e.Dst))
	s.logger = s.logger.With("handlerState", s.sm.Current())
}

func (s *MonitorItem) enterFailure(c context.Context, e *fsm.Event) {
	s.logger.Warn("encountered error", "err", e.Args[0])

	if s.debridID != "" {
		err := debrid.Remove(s.debridID)
		if err != nil {
			s.logger.Error("failed to remove from debrid", "err", err)
		}
		s.logger.Info("removed from debrid")
	}

	err := os.Remove(s.processingTorrent.FullPath)
	if err != nil {
		s.logger.Error("failed to remove from processing", "err", err)
	}

	s.removeFromSonarr()
	s.logger.Info("removed from sonarr")
}

func (s *MonitorItem) checkRequiredParams(c context.Context, e *fsm.Event) bool {
	requiredFields := StateRequiredFields[e.FSM.Current()]
	err := s.validateFields(requiredFields...)
	if err != nil {
		s.sm.Event(c, "failed", err)
		return false
	}
	return true
}

func (s *MonitorItem) enterProcessing(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}
	s.logger.Info("moving to processing")

	toProcess, err := torrents.NewFileToProcess(s.ingestedPath, s.config.ProcessingPath)
	if err != nil {
		if err := s.sm.Event(c, "failed", err); err != nil {
			s.logger.Error(fmt.Sprintf("event transition %s failed", "failed"), "err", err)
			return
		}
		return // Need to check if there is a way not to do this
	}

	s.setProcessingTorrent(toProcess)
	if err := s.sm.Event(c, "addToDebrid"); err != nil {
		s.logger.Error(fmt.Sprintf("event transition %s failed", "addToDebrid"), "err", err)
		return
	}
}

func (s *MonitorItem) enterAddingToDebrid(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}
	switch s.processingTorrent.FileType {
	case torrents.TorrentFile:
		s.logger.Info("adding torrent file to debrid")
		// TODO: Finish handling here - need to find a torrent file to test with
		torrentResponse, err := debrid.AddTorrent(s.processingTorrent.FullPath)
		if err != nil {
			s.sm.Event(c, "failed", err)
			return
		}
		s.setDebridID(torrentResponse.ID)
	case torrents.Magnet:
		s.logger.Info("getting magnet link")
		magnetLink, err := s.processingTorrent.GetMagnetLink()
		if err != nil {
			s.sm.Event(c, "failed", err)
			return
		}

		s.logger.Info("adding magnet to debrid")
		magnetResponse, err := debrid.AddMagnet(magnetLink)
		if err != nil {
			s.sm.Event(c, "failed", err)
			return
		}
		s.setDebridID(magnetResponse.ID)
	}

	if err := s.sm.Event(c, "checkDebridState"); err != nil {
		s.logger.Error(fmt.Sprintf("event transition %s failed", "checkDebridState"), "err", err)
		return
	}
}

func (s *MonitorItem) enterDebridProcessing(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}

	torrentInfo, err := debrid.GetInfo(s.debridID)
	if err != nil {
		s.sm.Event(c, "failed", err)
		return
	}

	s.logger = s.logger.With("debridStatus", torrentInfo.Status)
	s.logger.Debug("handling debrid status")

	switch torrentInfo.Status {
	case debrid.WaitingFileSelection:
		err := s.selectDebridFiles()
		if err != nil {
			s.sm.Event(c, "failed", err)
			return
		}

		if err := s.sm.Event(c, "retryDebridProcessing"); err != nil {
			s.logger.Error(fmt.Sprintf("event transition %s failed", "retryDebridProcessing"), "err", err)
			return
		}
		return
	case debrid.Queued:
		if err := s.sm.Event(c, "retryDebridProcessing"); err != nil {
			s.logger.Error(fmt.Sprintf("event transition %s failed", "retryDebridProcessing"), "err", err)
			return
		}
		return
	case debrid.Downloading:
		// TODO: Flag this as only if instant_availabiltiy is desired
		// If it is downloading I will need something to periodically check debrid waiting for completion,
		// then can be moved to the other monitor
		s.sm.Event(c, "failed", errors.New("not instantly available"))
		return
	case debrid.Downloaded:
		s.addToDebridMonitor(torrentInfo)
		if err := s.sm.Event(c, "complete"); err != nil {
			s.logger.Error(fmt.Sprintf("event transition %s failed", "complete"), "err", err)
			return
		}
		return
	default:
		s.sm.Event(c, "failed", errors.New(fmt.Sprintf("Unexpected debrid status - %s", torrentInfo.Status)))
		return
	}
}

func (s *MonitorItem) enterCompleted(c context.Context, _ *fsm.Event) {
	s.logger.Info("finished handling")
}

func (s *MonitorItem) selectDebridFiles() error {
	s.logger.Debug("selecting all files")
	err := debrid.SelectFiles(s.debridID, []string{})
	if err != nil {
		return err
	}

	return nil
}

func (s *MonitorItem) waitToRetryDebridProcessing(c context.Context, e *fsm.Event) {
	time.Sleep(1 * time.Second)

	if err := s.sm.Event(c, "checkDebridState"); err != nil {
		s.logger.Error(fmt.Sprintf("event transition %s failed", "checkDebridState"), "err", err)
		return
	}
}

func (s *MonitorItem) monitorSuccessCallback() error {
	_, err := s.sonarrClient.RefreshMonitoredDownloads()
	// TODO: Confirm refresh happened
	return err
}

func (s *MonitorItem) monitorFailureCallback() {
}

func (s *MonitorItem) addToDebridMonitor(torrentInfo debrid.GetInfoResponse) {
	s.logger = s.logger.With("torrentFilename", torrentInfo.Filename)
	s.logger = s.logger.With("sonarrCompletedDir", s.config.CompletedPath)
	s.logger = s.logger.With("sonarrProcessingPath", s.processingTorrent.FullPath)

	s.logger.Info("adding to monitor")
	debridMonitor.MonitorForDebridFiles(debridMonitor.MonitorConfig{
		Filename:         torrentInfo.Filename,
		OriginalFilename: torrentInfo.OriginalFilename,
		CompletedDir:     s.config.CompletedPath,
		Service:          arr.Sonarr,
		ProcessingPath:   s.processingTorrent.FullPath,
		Callbacks: debridMonitor.Callbacks{
			Success: func() error { return s.monitorSuccessCallback() },
			Failure: func() { s.monitorFailureCallback() },
		},
	}, s.logger)
}

func (s *MonitorItem) removeFromSonarr() {
	hash, err := s.processingTorrent.GetMagnetHash()
	if err != nil {
		s.logger.Error("failed to get hash")
		return
	}
	s.logger.With("magnetHash", hash)

	history, err := s.sonarrClient.SonarrGetHistory(50)
	if err != nil {
		s.logger.Error("failed to get history")
		return
	}

	toRemove := slices.IndexFunc(history.Records, func(item arr.SonarrHistoryItem) bool {
		return item.EventType == arr.Grabbed && item.Data.TorrentInfoHash == hash
	})
	if toRemove == -1 {
		s.logger.Error("could not find hash in history")
		return
	}
	sonarrId := history.Records[toRemove].ID
	s.logger.With("sonarrID", sonarrId)

	err = s.sonarrClient.SonarrFailHistoryItem(sonarrId)
	if toRemove == -1 {
		s.logger.Error("failed to fail history item with id")
		return
	}
}
