package server

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewDevelopmentLogger(logLevel zapcore.Level) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.DisableCaller = true
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger
}
