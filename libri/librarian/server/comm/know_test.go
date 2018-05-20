package comm

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestAlwaysKnower_Know(t *testing.T) {
	k := NewAlwaysKnower()
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)

	assert.True(t, k.Know(peerID))
}
