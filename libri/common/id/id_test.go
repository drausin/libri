package id

import (
	"bytes"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"fmt"
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
			in:  bytes.Repeat([]byte{255}, Length),
			out: UpperBound,
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

func TestFromString_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	randID := NewPseudoRandom(rng)
	cases := []struct {
		in  string
		out ID
	}{
		{in: strings.Repeat("0", 64), out: LowerBound},          // 0
		{in: strings.Repeat("f", 64), out: UpperBound},          // 1
		{in: strings.Repeat("F", 64), out: UpperBound},          // 2
		{in: strings.Repeat("0", 62) + "01", out: FromInt64(1)}, // 3
		{in: randID.String(), out: randID},                      // 4
	}
	for i, c := range cases {
		info := fmt.Sprintf("case: %d", i)
		id, err := FromString(c.in)
		assert.Nil(t, err, info)
		assert.Equal(t, 0, c.out.Cmp(id), info)
	}
}

func TestFromString_err(t *testing.T) {
	cases := []struct {
		in string
	}{
		{in: "0"},                      // 0
		{in: strings.Repeat("0", 32)},  // 1
		{in: strings.Repeat("0", 128)}, // 2
		{in: "F"},                      // 3
		{in: strings.Repeat("F", 32)},  // 4
		{in: strings.Repeat("F", 128)}, // 5
		{in: strings.Repeat("g", 64)},  // 6
		{in: strings.Repeat("G", 64)},  // 7
	}
	for i, c := range cases {
		info := fmt.Sprintf("case: %d", i)
		id, err := FromString(c.in)
		assert.NotNil(t, err, info)
		assert.Nil(t, id, info)
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
	assert.Equal(t, strings.Repeat("00", Length-1)+"ff", FromInt(big.NewInt(255)).String())
	assert.Equal(t, strings.Repeat("00", Length), LowerBound.String())
	assert.Equal(t, strings.Repeat("ff", Length), UpperBound.String())
}

func TestID_Bytes(t *testing.T) {
	cases := []struct {
		x        ID
		expected []byte
	}{
		{FromInt64(0), bytes.Repeat([]byte{0}, Length)},
		{FromInt64(1), append(bytes.Repeat([]byte{0}, Length-1), []byte{1}...)},
		{FromInt64(255), append(bytes.Repeat([]byte{0}, Length-1), []byte{255}...)},
		{FromBytes([]byte{255}), append(bytes.Repeat([]byte{0}, Length-1), []byte{255}...)},
		{FromInt64(65535), append(bytes.Repeat([]byte{0}, Length-2), []byte{255, 255}...)},
		{
			FromBytes([]byte{255, 255}),
			append(bytes.Repeat([]byte{0}, Length-2), []byte{255, 255}...),
		},
		{FromBytes(bytes.Repeat([]byte{255}, Length)), bytes.Repeat([]byte{255}, Length)},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, c.x.Bytes())
	}
}
