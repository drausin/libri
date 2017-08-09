package errors

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMaybePanic(t *testing.T) {
	MaybePanic(nil)
	assert.Panics(t, func() {
		MaybePanic(errors.New("some error"))
	})
}

func TestMonitorRunningErrorCount(t *testing.T) {
	errs := make(chan error, 8)
	fatal := make(chan error)
	maxRunningErrRate := float32(0.1)
	queueSize := 100
	maxRunningErrCount := int(float32(maxRunningErrRate) * float32(queueSize))
	lg := zap.NewNop()

	go MonitorRunningErrors(errs, fatal, queueSize, maxRunningErrRate, lg)

	// check get fatal error when go over threshold
	for c := 0; c < maxRunningErrCount; c++ {
		errs <- errors.New("some To error")
	}
	fataErr := <-fatal
	assert.Equal(t, ErrTooManyErrs, fataErr)

	go MonitorRunningErrors(errs, fatal, queueSize, maxRunningErrRate, lg)

	// check don't get fatal error when below threshold
	for c := 0; c < 200; c++ {
		var err error
		if c%25 == 0 {
			err = errors.New("some To error")
		}
		errs <- err
	}

	var fatalErr error
	select {
	case fatalErr = <-fatal:
	default:
	}
	assert.Nil(t, fatalErr)
}
