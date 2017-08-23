package ecid

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFromStored_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		original := NewPseudoRandom(rng)
		stored := ToStored(original)
		retrieved, err := FromStored(stored)

		assert.Nil(t, err)
		assert.Equal(t, original.Bytes(), retrieved.Bytes())
		assert.Equal(t, original.Key().D.Bytes(), retrieved.Key().D.Bytes())
	}
}

func TestToFromStored_curveErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	original := NewPseudoRandom(rng)
	stored := ToStored(original)
	stored.Curve = "other curve"
	retrieved, err := FromStored(stored)

	assert.NotNil(t, err)
	assert.Nil(t, retrieved)
}

func TestToFromStored_pubKeyErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	original := NewPseudoRandom(rng)
	stored := ToStored(original)
	stored.X = []byte("the wrong X coord")
	retrieved, err := FromStored(stored)

	assert.NotNil(t, err)
	assert.Nil(t, retrieved)
}
