package errors

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/pkg/errors"
)

func TestMaybePanic(t *testing.T) {
	MaybePanic(nil)
	assert.Panics(t, func() {
		MaybePanic(errors.New("some error"))
	})
}