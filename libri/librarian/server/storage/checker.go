package storage

import (
	"errors"
	"fmt"
)

// Checker checks that a key or value is value.
type Checker interface {
	// Check that a key or value is valid.
	Check(x []byte) error
}

// NewEmptyChecker creates a new Checker instance that ensures values have non-zero length.
func NewEmptyChecker() Checker {
	return &emptyChecker{}
}

type emptyChecker struct{}

func (ec *emptyChecker) Check(x []byte) error {
	if x == nil {
		return errors.New("must not be nil")
	}
	if len(x) == 0 {
		return errors.New("must have non-zero length")
	}
	return nil
}

// NewMaxLengthChecker creates a new Checker that ensures that values are not empty and have
// length less <= max length.
func NewMaxLengthChecker(max int) Checker {
	return &maxLengthChecker{
		max: max,
		ec:  NewEmptyChecker(),
	}
}

type maxLengthChecker struct {
	// max length allowed
	max int

	// empty checker
	ec Checker
}

func (lc *maxLengthChecker) Check(x []byte) error {
	if err := lc.ec.Check(x); err != nil {
		return err
	}
	if len(x) > lc.max {
		return fmt.Errorf("must have length <= %v, actual length = %v", lc.max, len(x))
	}
	return nil
}

// NewExactLengthChecker creates a Checker instance that ensures that values are not empty and
// have a specified length.
func NewExactLengthChecker(length int) Checker {
	return &exactLengthChecker{
		length: length,
		ec:     NewEmptyChecker(),
	}
}

type exactLengthChecker struct {
	// exact length allowed.
	length int

	// empty checker
	ec Checker
}

func (lc *exactLengthChecker) Check(x []byte) error {
	if err := lc.ec.Check(x); err != nil {
		return err
	}
	if len(x) != lc.length {
		return fmt.Errorf("must have length = %v, actual length = %v", lc.length, len(x))
	}
	return nil
}
