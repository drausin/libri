package storage

import (
	"bytes"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/stretchr/testify/assert"
)

func TestServerStorerLoader_StoreLoad_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	cases := []struct {
		key   []byte
		value []byte
	}{
		{bytes.Repeat([]byte{0}, cid.Length), []byte{0}},
		{cid.NewPseudoRandom(rng).Bytes(), []byte("test value")},
	}

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	gsl := NewServerStorerLoader(NewKVDBStorerLoader(kvdb))

	for _, c := range cases {
		err := gsl.Store(c.key, c.value)
		assert.Nil(t, err)

		loaded, err := gsl.Load(c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)
	}
}

func TestServerStorerLoader_StoreLoad_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte(nil), []byte{0}},
		{[]byte(""), []byte("test value")},
		{[]byte("test key"), []byte("")},
		{[]byte("test key"), []byte{255, 255, 255}},
	}

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	gsl := NewServerStorerLoader(NewKVDBStorerLoader(kvdb))

	for _, c := range cases {
		err := gsl.Store(c.key, c.value)
		assert.NotNil(t, err)

		_, err = gsl.Load(c.key)
		assert.NotNil(t, err)
	}
}

func TestKeySizeChecker_Check_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	kc := NewKeyChecker()
	cases := [][]byte{
		cid.NewPseudoRandom(rng).Bytes(),
		bytes.Repeat([]byte{0}, cid.Length),
		bytes.Repeat([]byte{255}, cid.Length),
	}
	for _, c := range cases {
		assert.Nil(t, kc.Check(c))
	}
}

func TestKeySizeChecker_Check_err(t *testing.T) {
	kc := NewKeyChecker()
	cases := [][]byte{
		nil,
		[]byte(nil),
		[]byte{},
		[]byte(""),
		[]byte{0}, // also too short
		[]byte("key too short"),
		bytes.Repeat([]byte{255}, 45), // too long
	}
	for _, c := range cases {
		assert.NotNil(t, kc.Check(c))
	}
}
