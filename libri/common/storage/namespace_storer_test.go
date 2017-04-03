package storage

import (
	"bytes"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
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

func TestDocumentNamespaceStorerLoader_StoreLoad_ok(t *testing.T) {
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	dsl := NewDocumentKVDBStorerLoader(kvdb)

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

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	dsl := NewDocumentKVDBStorerLoader(kvdb)

	// check invalid document returns error
	value1, _ := api.NewTestDocument(rng)
	value1.Contents.(*api.Document_Entry).Entry.AuthorPublicKey = nil
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	err = dsl.Store(key1, value1)
	assert.NotNil(t, err)

	// check bad key returns error
	value2, _ := api.NewTestDocument(rng)
	key2 := cid.NewPseudoRandom(rng)
	err = dsl.Store(key2, value2)
	assert.NotNil(t, err)
}

type fixedStorerLoader struct {
	storeErr  error
	loadValue []byte
	loadErr   error
}

func (fsl *fixedStorerLoader) Store(namespace []byte, key []byte, value []byte) error {
	return fsl.storeErr
}

func (fsl *fixedStorerLoader) Load(namespace []byte, key []byte) ([]byte, error) {
	return fsl.loadValue, fsl.loadErr
}

func TestDocumentStorerLoader_Load_empty(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := cid.NewPseudoRandom(rng)
	dsl1 := NewDocumentStorerLoader(&fixedStorerLoader{
		loadValue: nil, // simulates empty/missing value
	})

	// check that load returns nil
	value, err := dsl1.Load(key)
	assert.Nil(t, value)
	assert.Nil(t, err)
}

func TestDocumentStorerLoader_Load_loadErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := cid.NewPseudoRandom(rng)
	dsl1 := NewDocumentStorerLoader(&fixedStorerLoader{
		loadErr: errors.New("some load error"),
	})

	// check that load error propagates up
	_, err := dsl1.Load(key)
	assert.NotNil(t, err)
}

func TestDocumentStorerLoader_Load_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value, _ := api.NewTestDocument(rng)
	key := cid.NewPseudoRandom(rng)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	dsl := NewDocumentKVDBStorerLoader(kvdb)

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
	key := cid.NewPseudoRandom(rng)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	dsl := NewDocumentKVDBStorerLoader(kvdb)

	// hackily put a value with a non-hash key; should never happen in the wild
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	err = kvdb.Put(append(Documents, key.Bytes()...), valueBytes)
	assert.Nil(t, err)

	// check ValidateDocument error propagates up
	_, err = dsl.Load(key)
	assert.NotNil(t, err)
}

