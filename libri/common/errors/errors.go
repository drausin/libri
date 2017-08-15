package errors

import (
	"errors"

	"go.uber.org/zap"
)

// ErrTooManyErrs indicates when too many errors have occurred.
var ErrTooManyErrs = errors.New("too many errors")

// MaybePanic panics if the argument is not nil. It is useful for wrapping error-only return
// functions known to only return nil values.
func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

// MonitorRunningErrors reads (possibly nil) errors from errs and sends an error to fatal if
// the running non-nil error rate gets above maxRunningErrRate.
func MonitorRunningErrors(
	errs chan error, fatal chan error, queueSize int, maxRunningErrRate float32, logger *zap.Logger,
) {
	maxRunningErrCount := int(maxRunningErrRate * float32(queueSize))

	// fill error queue with non-errors
	runningErrs := make(chan error, queueSize)
	for c := 0; c < queueSize; c++ {
		runningErrs <- nil
	}

	// consume from errs and keep running error count; send fatal error if ever above threshold
	runningNErrs := 0
	for latestErr := range errs {
		if latestErr != nil {
			runningNErrs++
			logger.Debug("received non-fatal error", zap.Error(latestErr))
			if runningNErrs >= maxRunningErrCount {
				fatal <- ErrTooManyErrs
				return
			}
		}
		if earliestErr := <-runningErrs; earliestErr != nil {
			runningNErrs--
		}
		runningErrs <- latestErr
	}
}
