package monitor

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
	"github.com/samjwillis97/sams-blackhole/internal/logger"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

const (
	loggerName = "sonarr"
)

var StateRequiredFields = map[string][]string{
	"processing":       {"IngestedPath"},
	"addingToDebrid":   {"ProcessingTorrent"},
	"debridProcessing": {"debridID"},
}

// Still think there must be a better way of handling data between states
type SonarrTorrent struct {
	IngestedPath      string
	ProcessingTorrent torrents.ToProcess

	debridID    string
	timeoutTime time.Time
	prettyName  string
	logger      *slog.Logger

	FSM *fsm.FSM
}

func new() *SonarrTorrent {
	s := &SonarrTorrent{
		// Get level from config
		logger: slog.New(logger.NewHandler(loggerName, &slog.HandlerOptions{Level: slog.LevelDebug})).With(slog.String("name", loggerName)),
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

	s.FSM = fsm.NewFSM(
		"new",
		events,
		callbacks,
	)

	return s
}

func NewSonarrTorrent(filepath string) *SonarrTorrent {
	torrentItem := new()
	torrentItem.IngestedPath = filepath
	return torrentItem
}

func SonarrTorrentFromProcessing(torrent torrents.ToProcess) *SonarrTorrent {
	torrentItem := new()
	torrentItem.ProcessingTorrent = torrent
	return torrentItem
}

func (s *SonarrTorrent) validateFields(requiredFields ...string) error {
	fieldErrors := []string{}
	for _, field := range requiredFields {
		switch field {
		case "IngestedPath":
			if s.IngestedPath == "" {
				fieldErrors = append(fieldErrors, "IngestedPath is not set")
			}
		case "ProcessingTorrent":
			if (s.ProcessingTorrent == torrents.ToProcess{}) {
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

func (s *SonarrTorrent) enterState(c context.Context, e *fsm.Event) {
	if s.timeoutTime.IsZero() {
		s.timeoutTime = time.Now().Add(30 * time.Second)
	}

	if time.Now().After(s.timeoutTime) {
		s.FSM.Event(c, "failed", errors.New("timed out"))
	}

	s.logger.Debug(fmt.Sprintf("entering %s", e.Dst), s.getLogContext()...)
}

func (s *SonarrTorrent) enterFailure(c context.Context, e *fsm.Event) {
	s.logger.Warn("encountered error", s.getLogContext("err", e.Args[0])...)

	if s.debridID != "" {
		err := debrid.Remove(s.debridID)
		if err != nil {
			s.logger.Warn("failed to remove from debrid", s.getLogContext("err", err)...)
		}
		s.logger.Info("removed from debrid", s.getLogContext()...)
	}

	err := os.Remove(s.ProcessingTorrent.FullPath)
	if err != nil {
		s.logger.Warn("failed to remove from processing", s.getLogContext("err", err)...)
	}

	s.removeFromSonarr()
	s.logger.Info("removed from sonarr", s.getLogContext()...)
}

func (s *SonarrTorrent) checkRequiredParams(c context.Context, e *fsm.Event) bool {
	requiredFields := StateRequiredFields[e.FSM.Current()]
	err := s.validateFields(requiredFields...)
	if err != nil {
		s.FSM.Event(c, "failed", err)
		return false
	}
	return true
}

func (s *SonarrTorrent) enterProcessing(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}
	s.logger.Info("moving to processing", s.getLogContext()...)

	sonarrConfig := config.GetAppConfig().Sonarr
	toProcess, err := torrents.NewFileToProcess(s.IngestedPath, sonarrConfig.ProcessingPath)
	if err != nil {
		if err := s.FSM.Event(c, "failed", err); err != nil {
			s.logger.Warn(fmt.Sprintf("event transition %s failed", "failed"), s.getLogContext("err", err)...)
			return
		}
		return // Need to check if there is a way not to do this
	}

	s.ProcessingTorrent = toProcess
	if err := s.FSM.Event(c, "addToDebrid"); err != nil {
		s.logger.Warn(fmt.Sprintf("event transition %s failed", "addToDebrid"), s.getLogContext("err", err)...)
		return
	}
}

func (s *SonarrTorrent) enterAddingToDebrid(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}
	switch s.ProcessingTorrent.FileType {
	case torrents.TorrentFile:
		s.logger.Info("adding torrent file to debrid", s.getLogContext()...)
		// TODO: Finish handling here - need to find a torrent file to test with
		torrentResponse, err := debrid.AddTorrent(s.ProcessingTorrent.FullPath)
		if err != nil {
			s.FSM.Event(c, "failed", err)
			return
		}
		s.debridID = torrentResponse.ID
	case torrents.Magnet:
		s.logger.Info("getting magnet link", s.getLogContext()...)
		magnetLink, err := s.ProcessingTorrent.GetMagnetLink()
		if err != nil {
			s.FSM.Event(c, "failed", err)
			return
		}

		s.logger.Info("adding magnet to debrid", s.getLogContext()...)
		magnetResponse, err := debrid.AddMagnet(magnetLink)
		if err != nil {
			s.FSM.Event(c, "failed", err)
			return
		}

		s.debridID = magnetResponse.ID
	}

	if err := s.FSM.Event(c, "checkDebridState"); err != nil {
		s.logger.Warn(fmt.Sprintf("event transition %s failed", "checkDebridState"), s.getLogContext("err", err)...)
		return
	}
}

func (s *SonarrTorrent) enterDebridProcessing(c context.Context, e *fsm.Event) {
	if success := s.checkRequiredParams(c, e); !success {
		return
	}

	torrentInfo, err := debrid.GetInfo(s.debridID)
	if err != nil {
		s.FSM.Event(c, "failed", err)
		return
	}

	s.logger.Debug("handling debrid status", s.getLogContext("status", torrentInfo.Status)...)
	switch torrentInfo.Status {
	case debrid.WaitingFileSelection:
		err := s.selectDebridFiles()
		if err != nil {
			s.FSM.Event(c, "failed", err)
			return
		}

		if err := s.FSM.Event(c, "retryDebridProcessing"); err != nil {
			s.logger.Warn(fmt.Sprintf("event transition %s failed", "retryDebridProcessing"), s.getLogContext("err", err)...)
			return
		}
		return
	case debrid.Queued:
		if err := s.FSM.Event(c, "retryDebridProcessing"); err != nil {
			s.logger.Warn(fmt.Sprintf("event transition %s failed", "retryDebridProcessing"), s.getLogContext("err", err)...)
			return
		}
		return
	case debrid.Downloading:
		// TODO: Flag this as only if instant_availabiltiy is desired
		// If it is downloading I will need something to periodically check debrid waiting for completion,
		// then can be moved to the other monitor
		s.FSM.Event(c, "failed", errors.New("not instantly available"))
		return
	case debrid.Downloaded:
		s.addToDebridMonitor(torrentInfo)
		if err := s.FSM.Event(c, "complete"); err != nil {
			s.logger.Warn(fmt.Sprintf("event transition %s failed", "complete"), s.getLogContext("err", err)...)
			return
		}
		return
	default:
		s.FSM.Event(c, "failed", errors.New(fmt.Sprintf("Unexpected debrid status - %s", torrentInfo.Status)))
		return
	}
}

func (s *SonarrTorrent) enterCompleted(c context.Context, _ *fsm.Event) {
	s.logger.Info("finished handling", s.getLogContext()...)
}

func (s *SonarrTorrent) selectDebridFiles() error {
	s.logger.Debug("selecting all files", s.getLogContext()...)
	err := debrid.SelectFiles(s.debridID, []string{})
	if err != nil {
		return err
	}

	return nil
}

func (s *SonarrTorrent) waitToRetryDebridProcessing(c context.Context, e *fsm.Event) {
	time.Sleep(1 * time.Second)

	if err := s.FSM.Event(c, "checkDebridState"); err != nil {
		s.logger.Warn(fmt.Sprintf("event transition %s failed", "checkDebridState"), s.getLogContext("err", err)...)
		return
	}
}

func (s *SonarrTorrent) addToDebridMonitor(torrentInfo debrid.GetInfoResponse) {
	s.logger.Info("adding to monitor", s.getLogContext("torrentName", torrentInfo.Filename)...)
	sonarrConfig := config.GetAppConfig().Sonarr
	MonitorForDebridFiles(MonitorConfig{
		Filename:         torrentInfo.Filename,
		OriginalFilename: torrentInfo.OriginalFilename,
		CompletedDir:     sonarrConfig.CompletedPath,
		Service:          arr.Sonarr,
		ProcessingPath:   s.ProcessingTorrent.FullPath,
	})
}

func (s *SonarrTorrent) removeFromSonarr() {
	hash, err := s.ProcessingTorrent.GetMagnetHash()
	if err != nil {
		s.logger.Warn("failed to get hash", s.getLogContext()...)
		return
	}

	history, err := arr.SonarrGetHistory(50)
	if err != nil {
		s.logger.Warn("failed to get history", s.getLogContext("hash", hash)...)
		return
	}

	toRemove := slices.IndexFunc(history.Records, func(item arr.SonarrHistoryItem) bool {
		return item.EventType == arr.Grabbed && item.Data.TorrentInfoHash == hash
	})
	if toRemove == -1 {
		s.logger.Warn("could not find hash in history", s.getLogContext("hash", hash)...)
		return
	}

	err = arr.SonarrFailHistoryItem(history.Records[toRemove].ID)
	if toRemove == -1 {
		s.logger.Warn("failed to fail history item with id", s.getLogContext("sonarrId", history.Records[toRemove].ID)...)
		return
	}
}

func (s *SonarrTorrent) getLogContext(additional ...any) []any {
	context := []any{}
	context = append(context, additional...)

	if s.IngestedPath != "" {
		context = append(context, "ingested")
		context = append(context, s.IngestedPath)
	}

	if (s.ProcessingTorrent != torrents.ToProcess{}) {
		context = append(context, "processing")
		context = append(context, s.ProcessingTorrent.FullPath)
	}

	if s.debridID != "" {
		context = append(context, "debridId")
		context = append(context, s.debridID)
	}

	context = append(context, "currentState")
	context = append(context, s.FSM.Current())

	return context
}
