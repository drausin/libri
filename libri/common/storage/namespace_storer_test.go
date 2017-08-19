package storage

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"sync"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestServerClientSL_StoreLoad_ok(t *testing.T) {
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

func TestServerClientSL_Store_err(t *testing.T) {
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

func TestServerClientSL_Load_err(t *testing.T) {
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

func TestDocumentSLD_StoreLoadDelete_ok(t *testing.T) {
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsld := NewDocumentSLD(kvdb)

	rng := rand.New(rand.NewSource(0))
	value1, key := api.NewTestDocument(rng)

	err = dsld.Store(key, value1)
	assert.Nil(t, err)

	value2, err := dsld.Load(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, value2)

	err = dsld.Delete(key)
	assert.Nil(t, err)

	value3, err := dsld.Load(key)
	assert.Nil(t, err)
	assert.Nil(t, value3)
}

func TestDocumentSLD_Store_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsld := NewDocumentSLD(kvdb)

	// check invalid document returns error
	value1, _ := api.NewTestDocument(rng)
	value1.Contents.(*api.Document_Entry).Entry.AuthorPublicKey = nil
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	err = dsld.Store(key1, value1)
	assert.NotNil(t, err)

	// check bad key returns error
	value2, _ := api.NewTestDocument(rng)
	key2 := id.NewPseudoRandom(rng)
	err = dsld.Store(key2, value2)
	assert.NotNil(t, err)
}

func TestDocumentSLD_Iterate(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	dsld := NewDocumentSLD(kvdb)

	nDocs := 3
	vals := make(map[string]*api.Document)
	for c := 0; c < nDocs; c++ {
		val, key := api.NewTestDocument(rng)
		vals[key.String()] = val
		err := dsld.Store(key, val)
		assert.Nil(t, err)
	}

	nIters := 0
	callback := func(key id.ID, value []byte) {
		nIters++
		expected, in := vals[key.String()]
		assert.True(t, in)
		expectedBytes, err2 := proto.Marshal(expected)
		assert.Nil(t, err2)
		assert.Equal(t, expectedBytes, value)
	}
	err = dsld.Iterate(make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, len(vals), nIters)
}

func TestDocumentSLD_Metrics(t *testing.T) {
	m1 := &DocumentMetrics{
		NDocuments: 1,
		TotalSize:  2,
	}
	dsld := &documentSLD{
		metrics: m1,
		mu:      new(sync.Mutex),
	}
	m2 := dsld.Metrics()
	assert.Equal(t, m1, m2)

	// check m2 is a clone of m1 (but not same)
	m1.NDocuments = 3
	assert.NotEqual(t, m1, m2)
}

func TestDocumentSLD_Load_empty(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)
	dsl1 := &documentSLD{
		sld: &TestSLD{
			Bytes: nil, // simulates missing/empty value
		},
	}

	// check that load returns nil
	value, err := dsl1.Load(key)
	assert.Nil(t, value)
	assert.Nil(t, err)
}

func TestDocumentSLD_Load_loadErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)
	dsl1 := &documentSLD{
		sld: &TestSLD{
			LoadErr: errors.New("some load error"),
		},
	}

	// check that load error propagates up
	_, err := dsl1.Load(key)
	assert.NotNil(t, err)
}

func TestDocumentSLD_Load_checkErr(t *testing.T) {
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

func TestDocumentSLD_Load_validateDocumentErr(t *testing.T) {
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

func TestDocumentSLD_Mac(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)
	value := []byte("some value")
	macKey := []byte("some mac key")

	// check mac of present value is well-formed
	dsld := &documentSLD{
		sld: &TestSLD{Bytes: value},
		c:   &fixedKVChecker{},
	}
	mac, err := dsld.Mac(key, macKey)
	assert.Nil(t, err)
	assert.Equal(t, 32, len(mac))

	// check mac of missing value is nil (but so is error)
	dsld = &documentSLD{
		sld: &TestSLD{},
		c:   &fixedKVChecker{},
	}
	mac, err = dsld.Mac(key, macKey)
	assert.Nil(t, err)
	assert.Nil(t, mac)

	// check inner SLD error bubbles up
	dsld = &documentSLD{
		sld: &TestSLD{LoadErr: errors.New("some Load error")},
		c:   &fixedKVChecker{},
	}
	mac, err = dsld.Mac(key, macKey)
	assert.NotNil(t, err)
	assert.Nil(t, mac)
}

func TestDocumentSLD_Delete_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := id.NewPseudoRandom(rng)

	// check inner Load error bubbles up
	dsld := &documentSLD{
		sld: &TestSLD{LoadErr: errors.New("some Load error")},
	}
	err := dsld.Delete(key)
	assert.NotNil(t, err)

	// check inner delete error bubbles up
	dsld = &documentSLD{
		sld: &TestSLD{DeleteErr: errors.New("some Delete error")},
	}
	err = dsld.Delete(key)
	assert.NotNil(t, err)
}

type fixedKVChecker struct {
	err error
}

func (f *fixedKVChecker) Check(key []byte, value []byte) error {
	return f.err
}
