package subscribe

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestNewSubscription_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s, err := NewSubscription([][]byte{}, 0.5, [][]byte{}, 0.5, rng)
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, s.AuthorPublicKeys)
	assert.NotNil(t, s.ReaderPublicKeys)
}

func TestNewSubscription_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	cases := []struct {
		authorPubs [][]byte
		authorFp   float64
		readerPubs [][]byte
		readerFp   float64
	}{
		{nil, 0.5, [][]byte{}, 0.5},          // 0
		{[][]byte{}, -0.05, [][]byte{}, 0.5}, // 1
		{[][]byte{}, 0.0, [][]byte{}, 0.5},   // 2
		{[][]byte{}, 1.5, [][]byte{}, 0.5},   // 3
		{[][]byte{}, 0.5, nil, 0.5},          // 4
		{[][]byte{}, 0.5, [][]byte{}, -0.05}, // 5
		{[][]byte{}, 0.5, [][]byte{}, 0.0},   // 6
		{[][]byte{}, 0.5, [][]byte{}, 1.5},   // 7
	}
	for i, c := range cases {
		info := fmt.Sprintf("i: %d", i)
		s, err := NewSubscription(c.authorPubs, c.authorFp, c.readerPubs, c.readerFp, rng)
		assert.NotNil(t, err, info)
		assert.Nil(t, s, info)
	}
}

func TestNewAuthorSubscription(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s, err := NewAuthorSubscription([][]byte{}, 0.5, rng)
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, s.AuthorPublicKeys)
	assert.NotNil(t, s.ReaderPublicKeys)
}

func TestNewReaderSubscription(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s, err := NewReaderSubscription([][]byte{}, 0.5, rng)
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, s.AuthorPublicKeys)
	assert.NotNil(t, s.ReaderPublicKeys)
}

func TestNewFPSubscription(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s, err := NewFPSubscription(0.5, rng)
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, s.AuthorPublicKeys)
	assert.NotNil(t, s.ReaderPublicKeys)
}
