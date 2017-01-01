package db

import (
	"io/ioutil"
	"testing"

	"github.com/drausin/libri/libri/common"
	"github.com/stretchr/testify/assert"
)

var (
	rocksDBName = "kvdb-test-rocksdb"
)

// Test creating a new RocksDB instance.
func TestRocksDB_NewRocksDB(t *testing.T) {
	dir, err := ioutil.TempDir("", rocksDBName)
	assert.Nil(t, err)
	db, err := NewRocksDB(dir)
	assert.Nil(t, err)
	defer common.MaybePanic(db.Close())

	assert.Nil(t, err)
	assert.NotNil(t, db.wo)
	assert.NotNil(t, db.ro)
	assert.NotNil(t, db.rdb)
}

// Test putting and then getting a value works as expected.
func TestRocksDB_PutGet(t *testing.T) {
	dir, err := ioutil.TempDir("", rocksDBName)
	assert.Nil(t, err)
	db, err := NewRocksDB(dir)
	assert.Nil(t, err)
	key, value1 := []byte("key"), []byte("value1")

	assert.Nil(t, db.Put(key, value1))
	getValue1, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	common.MaybePanic(db.Close())
}

// Test a second put overwrites the value of the first.
func TestRocksDB_PutGetPutGet(t *testing.T) {
	dir, err := ioutil.TempDir("", rocksDBName)
	assert.Nil(t, err)
	db, err := NewRocksDB(dir)
	assert.Nil(t, err)
	key, value1, value2 := []byte("key"), []byte("value1"), []byte("value2")

	assert.Nil(t, db.Put(key, value1))
	getValue1, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	assert.Nil(t, db.Put(key, value2))
	getValue2, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value2, getValue2)

	common.MaybePanic(db.Close())
}

// Test deleting a put value.
func TestRocksDB_PutGetDeleteGet(t *testing.T) {
	dir, err := ioutil.TempDir("", rocksDBName)
	assert.Nil(t, err)
	db, err := NewRocksDB(dir)
	if err != nil {
		panic(err)
	}
	defer db.rdb.Close()
	key, value1 := []byte("key"), []byte("value1")

	assert.Nil(t, db.Put(key, value1))
	getValue1, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	assert.Nil(t, db.Delete(key))
	getValue2, err := db.Get(key)
	assert.Nil(t, err)
	assert.Nil(t, getValue2)
}
