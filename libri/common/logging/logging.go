package server

import (
	"github.com/drausin/libri/libri/common/errors"
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

// ErrArray is an array of errors
type ErrArray []error

// ToErrArray concerts a map of errors to an array of errors.
func ToErrArray(errMap map[string]error) ErrArray {
	errArray := make([]error, len(errMap))
	i := 0
	for _, err := range errMap {
		errArray[i] = err
		i++
	}
	return errArray
}

// MarshalLogArray marshals the array of errors.
func (errs ErrArray) MarshalLogArray(arr zapcore.ArrayEncoder) error {
	for _, err := range errs {
		arr.AppendString(err.Error())
	}
	return nil
}
