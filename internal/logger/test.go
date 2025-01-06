package logger

import (
	"github.com/rs/zerolog"
	"os"
)

type EventHandler struct {
	logger zerolog.Logger
}

type ServiceHandler struct {
	logger zerolog.Logger
}

func Main() {
	// Initialize the base logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	baseLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Example event data
	eventID := "213"
	eventLogger := baseLogger.With().Dict("event", zerolog.Dict().Str("id", eventID)).Logger()

	eventHandler := &EventHandler{logger: eventLogger}
	eventHandler.HandleEvent()
}

func (eh *EventHandler) HandleEvent() {
	// Example log in the event handler
	eh.logger.Info().Msg("event log")

	// Pass logger to service handler
	serviceHandler := &ServiceHandler{logger: eh.logger}
	serviceHandler.Process("processing")
}

func (sh *ServiceHandler) Process(state string) {
	// Add service-specific fields using namespace
	sh.logger = sh.logger.With().Dict("service", zerolog.Dict().Str("state", "processing")).Logger()
	sh.logger = sh.logger.With().Dict("service", zerolog.Dict().Str("id", "123123")).Logger()

	// Example log in the service handler
	sh.logger.Info().Msg("service log")
}
