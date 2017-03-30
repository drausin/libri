package server

import (
	"testing"
	"go.uber.org/zap"
	"github.com/stretchr/testify/assert"
)

func TestNewDevLogger(t *testing.T) {
	l := NewDevLogger(zap.DebugLevel)
	assert.NotNil(t, l)
}

func TestNewDevInfoLogger(t *testing.T) {
	l := NewDevInfoLogger()
	assert.NotNil(t, l)
}
