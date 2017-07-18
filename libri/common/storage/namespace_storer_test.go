package storage

import (
	"bytes"
	"math/rand"
	"testing"
	"errors"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestServerClientStorerLoader_StoreLoad_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	cases := []struct {
		key   []byte
		value []byte
	}{
		{bytes.Repeat([]byte{0}, id.Length), []byte{0}},
		{id.NewPseudoRandom(rng).Bytes(), []byte("test value")},
	}

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	ssl := NewServerSL(kvdb)
	csl := NewClientSL(kvdb)

	for _, c := range cases {
		err := ssl.Store(c.key, c.value)
		assert.Nil(t, err)
		err = csl.Store(c.key, c.value)
		assert.Nil(t, err)

		loaded, err := ssl.Load(c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)

		loaded, err = csl.Load(c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)
	}
}

func TestServerClientStorerLoader_Store_err(t *testing.T) {
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

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	ssl := NewServerSL(kvdb)
	csl := NewClientSL(kvdb)

	for _, c := range cases {
		err := ssl.Store(c.key, c.value)
		assert.NotNil(t, err)
		err = csl.Store(c.key, c.value)
		assert.NotNil(t, err)
	}
}

func TestServerClientStorerLoader_Load_err(t *testing.T) {
	cases := []struct {
		key   []byte
		value []byte
	}{
		{[]byte(nil), []byte{0}},                               // nil key
		{[]byte(""), []byte("test value")},                     // empty key
		{bytes.Repeat([]byte{255}, 257), []byte("test value")}, // too long key
	}

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	ssl := NewServerSL(kvdb)
	csl := NewServerSL(kvdb)

	for _, c := range cases {
		_, err = ssl.Load(c.key)
		assert.NotNil(t, err)
		_, err = csl.Load(c.key)
		assert.NotNil(t, err)
	}
}

func TestDocumentNamespaceStorerLoader_StoreLoad_ok(t *testing.T) {
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsl := NewDocumentSLD(kvdb)

	rng := rand.New(rand.NewSource(0))
	value1, key := api.NewTestDocument(rng)

	err = dsl.Store(key, value1)
	assert.Nil(t, err)

	value2, err := dsl.Load(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, value2)
}

func TestDocumentNamespaceStorerLoader_Store_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsl := NewDocumentSLD(kvdb)

	// check invalid document returns error
	value1, _ := api.NewTestDocument(rng)
	value1.Contents.(*api.Document_Entry).Entry.AuthorPublicKey = nil
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	err = dsl.Store(key1, value1)
	assert.NotNil(t, err)

	// check bad key returns error
	value2, _ := api.NewTestDocument(rng)
	key2 := id.NewPseudoRandom(rng)
	err = dsl.Store(key2, value2)
	assert.NotNil(t, err)
}

func TestDocumentStorerLoader_Load_empty(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)
	dsl1 := &documentSLD{
		sld: &fixedNamespaceSLD{
			loadValue: nil, // simulates missing/empty value
		},
	}

	// check that load returns nil
	value, err := dsl1.Load(key)
	assert.Nil(t, value)
	assert.Nil(t, err)
}

func TestDocumentStorerLoader_Load_loadErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)
	dsl1 := &documentSLD{
		sld: &fixedNamespaceSLD{
			loadErr: errors.New("some load error"),
		},
	}

	// check that load error propagates up
	_, err := dsl1.Load(key)
	assert.NotNil(t, err)
}

func TestDocumentStorerLoader_Load_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value, _ := api.NewTestDocument(rng)
	key := id.NewPseudoRandom(rng)
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsl := NewDocumentSLD(kvdb)

	// hackily put a value with a non-hash key; should never happen in the wild
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	err = kvdb.Put(append(Documents, key.Bytes()...), valueBytes)
	assert.Nil(t, err)

	// check Check error propagates up
	_, err = dsl.Load(key)
	assert.NotNil(t, err)
}

func TestDocumentStorerLoader_Load_validateDocumentErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value, _ := api.NewTestDocument(rng)
	value.Contents.(*api.Document_Entry).Entry.AuthorPublicKey = nil
	key := id.NewPseudoRandom(rng)
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsl := NewDocumentSLD(kvdb)

	// hackily put a value with a non-hash key; should never happen in the wild
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	err = kvdb.Put(append(Documents, key.Bytes()...), valueBytes)
	assert.Nil(t, err)

	// check ValidateDocument error propagates up
	_, err = dsl.Load(key)
	assert.NotNil(t, err)
}

type fixedNamespaceSLD struct {
	loadValue []byte
	storeErr  error
	loadErr   error
	deleteErr error
}

func (f *fixedNamespaceSLD) Store(key []byte, value []byte) error {
	return f.storeErr
}

func (f *fixedNamespaceSLD) Load(key []byte) ([]byte, error) {
	return f.loadValue, f.loadErr

}

func (f *fixedNamespaceSLD) Delete(key []byte) error {
	return f.deleteErr
}
