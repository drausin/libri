package goodwill

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestNaiveDoctor_Healthy(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	d := NewNaiveDoctor()
	assert.True(t, d.Healthy(id.NewPseudoRandom(rng)))
}
