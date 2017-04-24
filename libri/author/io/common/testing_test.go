package common

import (
	"math/rand"
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestNewCompressableBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nBytes := 128
	buf := NewCompressableBytes(rng, nBytes)
	assert.Equal(t, nBytes, buf.Len())
}
