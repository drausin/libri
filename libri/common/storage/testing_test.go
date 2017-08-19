package storage

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestTestSLD(t *testing.T) {
	key, value1, value2 := []byte{1, 2, 3}, []byte{4, 5, 6}, []byte{7, 8, 9}
	sld := &TestSLD{
		Bytes:      value1,
		LoadErr:    errors.New("some Load error"),
		IterateErr: errors.New("some Iterate error"),
		StoreErr:   errors.New("some Store error"),
		DeleteErr:  errors.New("some Delete error"),
	}

	bytes, err := sld.Load(key)
	assert.NotNil(t, err)
	assert.Equal(t, value1, bytes)

	err = sld.Iterate(nil, nil, nil, nil)
	assert.NotNil(t, err)

	err = sld.Store(key, value2)
	assert.NotNil(t, err)
	assert.Equal(t, value2, sld.Bytes)

	err = sld.Delete(key)
	assert.NotNil(t, err)
	assert.Nil(t, sld.Bytes)
}

func TestTestDocSLD(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	dsld := NewTestDocSLD()
	dsld.StoreErr = errors.New("some Store error")
	dsld.IterateErr = errors.New("some Iterate error")
	dsld.LoadErr = errors.New("some Load error")
	dsld.MacErr = errors.New("some Mac error")
	dsld.DeleteErr = errors.New("some Delete error")

	nDocs := 3
	keys := make([]id.ID, nDocs)
	for c := 0; c < nDocs; c++ {
		// kinda testing both happy and sad paths here at once, but...it's fine
		value1, key := api.NewTestDocument(rng)
		keys[c] = key
		err := dsld.Store(key, value1)
		assert.NotNil(t, err)

		value2, err := dsld.Load(key)
		assert.NotNil(t, err)
		assert.Equal(t, value1, value2)

		_, err = dsld.Mac(key, nil)
		assert.NotNil(t, err)
	}

	nIters := 0
	done := make(chan struct{})
	err := dsld.Iterate(done, func(key id.ID, value []byte) {
		nIters++
	})
	assert.NotNil(t, err)
	assert.Equal(t, nDocs, nIters)

	nIters = 0
	close(done)
	err = dsld.Iterate(done, func(key id.ID, value []byte) {
		nIters++
	})
	assert.NotNil(t, err)
	assert.Zero(t, nIters)

	for c := 0; c < nDocs; c++ {
		err = dsld.Delete(keys[c])
		assert.NotNil(t, err)
	}
	assert.Zero(t, len(dsld.Stored))
}
