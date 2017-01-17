package id

import (
	"bytes"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestID_FromBytes_ok(t *testing.T) {
	cases := []struct {
		in  []byte
		out ID
	}{
		{in: []byte{}, out: FromInt(big.NewInt(0))},
		{in: []byte{0}, out: FromInt(big.NewInt(0))},
		{in: []byte{1}, out: FromInt(big.NewInt(1))},
		{in: bytes.Repeat([]byte{0}, Length), out: FromInt(big.NewInt(0))},
		{
			// 256 one bits, or 2^256 - 1
			in: bytes.Repeat([]byte{255}, Length),
			out: FromInt(big.NewInt(0).Sub(big.NewInt(0).Lsh(big.NewInt(1), 256),
				big.NewInt(1))),
		},
	}
	for _, c := range cases {
		assert.Equal(t, 0, c.out.Cmp(FromBytes(c.in)))
	}
}

func TestFromBytes_panic(t *testing.T) {
	cases := [][]byte{
		bytes.Repeat([]byte{0}, Length+1),
		bytes.Repeat([]byte{255}, Length+1),
		bytes.Repeat([]byte{0}, 33),
		bytes.Repeat([]byte{255}, 33),
	}
	for _, c := range cases {
		assert.Panics(t, func() {
			FromBytes(c)
		})
	}
}

func TestNewRandom(t *testing.T) {
	for c := 0; c < 10; c++ {
		val := NewRandom()
		assert.True(t, val.Cmp(LowerBound) >= 0)
		assert.True(t, val.Cmp(UpperBound) <= 0)
	}
}

func TestNewPsuedoRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.Cmp(LowerBound) >= 0)
		assert.True(t, val.Cmp(UpperBound) <= 0)
	}
}

func TestDistance(t *testing.T) {
	cases := []struct {
		x   ID
		y   ID
		out *big.Int
	}{
		{x: FromInt(big.NewInt(0)), y: FromInt(big.NewInt(0)), out: big.NewInt(0)},
		{x: FromInt(big.NewInt(0)), y: FromInt(big.NewInt(1)), out: big.NewInt(1)},
		{x: FromInt(big.NewInt(0)), y: FromInt(big.NewInt(128)), out: big.NewInt(128)},
		{x: FromInt(big.NewInt(1)), y: FromInt(big.NewInt(255)), out: big.NewInt(254)},
		{x: FromInt(big.NewInt(2)), y: FromInt(big.NewInt(255)), out: big.NewInt(253)},
	}
	for _, c := range cases {
		assert.Equal(t, 0, c.x.Distance(c.y).Cmp(c.out), "x: %v, y: %v", c.x, c.y)
	}
}

func TestString(t *testing.T) {
	assert.Equal(t, strings.Repeat("00", Length), FromInt(big.NewInt(0)).String())
	assert.Equal(t, strings.Repeat("00", Length-1)+"01", FromInt(big.NewInt(1)).String())
	assert.Equal(t, strings.Repeat("00", Length-1)+"FF", FromInt(big.NewInt(255)).String())
	assert.Equal(t, strings.Repeat("00", Length), LowerBound.String())
	assert.Equal(t, strings.Repeat("FF", Length), UpperBound.String())
}
