package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type EventHandler struct {
	logger *zap.Logger
}

type ServiceHandler struct {
	logger *zap.Logger
}

func Main() {
	// Initialize the base logger
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "ts"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := config.Build()
	defer logger.Sync()

	// Example event data
	eventID := "213"
	eventLogger := logger.With(
		zap.Namespace("event"),
		zap.String("id", eventID),
	)

	eventHandler := &EventHandler{logger: eventLogger}
	eventHandler.HandleEvent()
}

func (eh *EventHandler) HandleEvent() {
	// Example log in the event handler
	eh.logger.Info("event log")

	// Pass logger to service handler
	serviceHandler := &ServiceHandler{logger: eh.logger}
	serviceHandler.Process("processing")
}

func (sh *ServiceHandler) Process(state string) {
	// Add service-specific fields using namespace
	serviceLogger := sh.logger.With(
		zap.Namespace("service"),
		zap.String("state", state),
	)

	// Example log in the service handler
	serviceLogger.Info("service log")
}

