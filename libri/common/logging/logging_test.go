package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewDevLogger(t *testing.T) {
	l := NewDevLogger(zap.DebugLevel)
	assert.NotNil(t, l)
}

func TestNewDevInfoLogger(t *testing.T) {
	l := NewDevInfoLogger()
	assert.NotNil(t, l)
}
