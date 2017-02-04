package storage

import (
	"bytes"
	"math/rand"
	"testing"

	"crypto/sha256"

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
	ssl := NewServerKVDBStorerLoader(kvdb)

	for _, c := range cases {
		err := ssl.Store(c.key, c.value)
		assert.Nil(t, err)

		loaded, err := ssl.Load(c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)
	}
}

func TestServerStorerLoader_Store_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte(nil), []byte{0}},                               // nil key
		{[]byte(""), []byte("test value")},                     // empty key
		{bytes.Repeat([]byte{255}, 257), []byte("test value")}, // too long key
		{[]byte("test key"), nil},                              // nil value
		{[]byte("test key"), []byte("")},                       // empty value
	}

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	ssl := NewServerKVDBStorerLoader(kvdb)

	for _, c := range cases {
		err := ssl.Store(c.key, c.value)
		assert.NotNil(t, err)
	}
}

func TestServerStorerLoader_Load_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte(nil), []byte{0}},                               // nil key
		{[]byte(""), []byte("test value")},                     // empty key
		{bytes.Repeat([]byte{255}, 257), []byte("test value")}, // too long key
	}

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	ssl := NewServerKVDBStorerLoader(kvdb)

	for _, c := range cases {
		_, err = ssl.Load(c.key)
		assert.NotNil(t, err)
	}
}

func TestKeyHashNamespaceStorerLoader_StoreLoad_ok(t *testing.T) {
	cases := [][]byte{
		[]byte("some value"),
		[]byte{255},
		bytes.Repeat([]byte{1}, 64),
	}
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	esl := NewEntriesKVDBStorerLoader(kvdb)

	for _, c := range cases {
		key := sha256.Sum256(c) // explicitly get the hash of the value as our key
		err := esl.Store(key[:], c)
		assert.Nil(t, err)

		value, err := esl.Load(key[:])
		assert.Nil(t, err)
		assert.Equal(t, c, value)
	}
}

func TestKeyHashNamespaceStorerLoader_Store_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte("some key"), []byte("some value")},
		{[]byte("some key"), []byte{255}},
		{bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{1}, 64)},
	}
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	esl := NewEntriesKVDBStorerLoader(kvdb)

	for _, c := range cases {
		err := esl.Store(c.key, c.value)
		assert.NotNil(t, err)
	}
}

func TestKeyHashNamespaceStorerLoader_Load_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte("some key"), []byte("some value")},
		{[]byte("some key"), []byte{255}},
		{bytes.Repeat([]byte{1}, 64), bytes.Repeat([]byte{1}, 64)},
	}
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	esl := NewEntriesKVDBStorerLoader(kvdb)

	for _, c := range cases {
		// hackily put a value with a non-hash key; should never happen in the wild
		kvdb.Put(append(Entries, c.key...), c.value)
		_, err := esl.Load(c.key)
		assert.NotNil(t, err)
	}
}
