package ecid

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFromStored(t *testing.T) {
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
