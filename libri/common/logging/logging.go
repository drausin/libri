package server

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewDevLogger creates a new logger with a given log level for use in development (i.e., not
// production).
func NewDevLogger(logLevel zapcore.Level) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.DisableCaller = true
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

// NewDevInfoLogger creates a new development logger at the INFO level.
func NewDevInfoLogger() *zap.Logger {
	return NewDevLogger(zap.InfoLevel)
}
