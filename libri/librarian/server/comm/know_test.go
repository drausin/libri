package comm

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestNeverKnower_Know(t *testing.T) {
	k := NewNeverKnower()
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)

	assert.False(t, k.Know(peerID))
}
