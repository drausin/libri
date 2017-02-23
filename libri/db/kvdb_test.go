package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test creating a new RocksDB instance.
func TestRocksDB_NewRocksDB(t *testing.T) {
	db, err := NewTempDirRocksDB()
	assert.Nil(t, err)
	defer db.Close()

	assert.NotNil(t, db.wo)
	assert.NotNil(t, db.ro)
	assert.NotNil(t, db.rdb)
}

// Test putting and then getting a value works as expected.
func TestRocksDB_PutGet(t *testing.T) {
	db, err := NewTempDirRocksDB()
	assert.Nil(t, err)
	defer db.Close()
	key, value1 := []byte("key"), []byte("value1")

	assert.Nil(t, db.Put(key, value1))
	getValue1, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)
}

func TestRocksDB_Get_err(t *testing.T) {
	db := &RocksDB{}
	value, err := db.Get([]byte("key"))
	assert.Nil(t, value)
	assert.NotNil(t, err)
}

// Test a second put overwrites the value of the first.
func TestRocksDB_PutGetPutGet(t *testing.T) {
	db, err := NewTempDirRocksDB()
	assert.Nil(t, err)
	defer db.Close()
	key, value1, value2 := []byte("key"), []byte("value1"), []byte("value2")

	assert.Nil(t, db.Put(key, value1))
	getValue1, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	assert.Nil(t, db.Put(key, value2))
	getValue2, err := db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value2, getValue2)
}

// Test deleting a put value.
func TestRocksDB_PutGetDeleteGet(t *testing.T) {
	db, err := NewTempDirRocksDB()
	assert.Nil(t, err)
	defer db.Close()
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
