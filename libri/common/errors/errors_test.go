package errors

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestMaybePanic(t *testing.T) {
	MaybePanic(nil)
	assert.Panics(t, func() {
		MaybePanic(errors.New("some error"))
	})
}
