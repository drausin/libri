package server

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/drausin/libri/libri/common/errors"
)

// NewDevLogger creates a new logger with a given log level for use in development (i.e., not
// production).
func NewDevLogger(logLevel zapcore.Level) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.DisableCaller = true
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	errors.MaybePanic(err)
	return logger
}

// NewDevInfoLogger creates a new development logger at the INFO level.
func NewDevInfoLogger() *zap.Logger {
	return NewDevLogger(zap.InfoLevel)
}

// NewProdLogger creates a new logger with a given log level for use in production.
func NewProdLogger(logLevel zapcore.Level) *zap.Logger {
	config := zap.NewProductionConfig()
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	errors.MaybePanic(err)
	return logger
}
