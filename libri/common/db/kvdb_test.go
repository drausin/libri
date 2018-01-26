package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRocksDB_NewRocksDB(t *testing.T) {
	db, cleanup, err := NewTempDirRocksDB()
	defer cleanup()
	defer db.Close()
	assert.Nil(t, err)

	assert.NotNil(t, db.wo)
	assert.NotNil(t, db.ro)
	assert.NotNil(t, db.rdb)
}

func TestRocksDB_PutGet(t *testing.T) {
	db, cleanup, err := NewTempDirRocksDB()
	defer cleanup()
	defer db.Close()
	assert.Nil(t, err)
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

func TestRocksDB_PutGetPutGet(t *testing.T) {
	db, cleanup, err := NewTempDirRocksDB()
	defer cleanup()
	defer db.Close()
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
}

func TestRocksDB_PutGetDeleteGet(t *testing.T) {
	db, cleanup, err := NewTempDirRocksDB()
	defer cleanup()
	defer db.Close()
	assert.Nil(t, err)
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

func TestRocksDB_Iterate(t *testing.T) {
	db, cleanup, err := NewTempDirRocksDB()
	defer cleanup()
	defer db.Close()
	assert.Nil(t, err)

	// add data
	vals := map[string][]byte{
		"key1": []byte("val1"),
		"key2": []byte("val2"),
		"key3": []byte("val3"),
	}
	for key, val := range vals {
		err = db.Put([]byte(key), val)
		assert.Nil(t, err)
	}

	// iterate through everything
	nIters := 0
	callback := func(key, value []byte) {
		nIters++
		expected, in := vals[string(key)]
		assert.True(t, in)
		assert.Equal(t, expected, value)
	}
	lb, ub := []byte("key0"), []byte("key9")
	err = db.Iterate(lb, ub, make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, len(vals), nIters)

	// iterate through only some
	nIters = 0
	lb, ub = []byte("key0"), []byte("key3")
	err = db.Iterate(lb, ub, make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, 2, nIters)

	// iterate through single value and send done signal
	nIters = 0
	done := make(chan struct{})
	callback = func(key, value []byte) {
		nIters++
		expected, in := vals[string(key)]
		assert.True(t, in)
		assert.Equal(t, expected, value)
		close(done)
	}
	lb, ub = []byte("key0"), []byte("key9")
	err = db.Iterate(lb, ub, done, callback)
	assert.Nil(t, err)
	assert.Equal(t, 1, nIters)
}

func TestMemory_PutGet(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()
	key, value := []byte("key"), []byte("value")

	assert.Nil(t, mdb.Put(key, value))
	getValue, err := mdb.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, getValue)
}

func TestMemory_PutGetPutGet(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()
	key, value1, value2 := []byte("key"), []byte("value1"), []byte("value2")

	assert.Nil(t, mdb.Put(key, value1))
	getValue1, err := mdb.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	assert.Nil(t, mdb.Put(key, value2))
	getValue2, err := mdb.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value2, getValue2)
}

func TestMemory_PutGetDeleteGet(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()
	key, value1 := []byte("key"), []byte("value1")

	assert.Nil(t, mdb.Put(key, value1))
	getValue1, err := mdb.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value1, getValue1)

	assert.Nil(t, mdb.Delete(key))
	getValue2, err := mdb.Get(key)
	assert.Nil(t, err)
	assert.Nil(t, getValue2)
}

func TestMemory_Iterate(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()

	// add data
	vals := map[string][]byte{
		"key1": []byte("val1"),
		"key2": []byte("val2"),
		"key3": []byte("val3"),
	}
	var err error
	for key, val := range vals {
		err = mdb.Put([]byte(key), val)
		assert.Nil(t, err)
	}

	// iterate through everything
	nIters := 0
	callback := func(key, value []byte) {
		nIters++
		expected, in := vals[string(key)]
		assert.True(t, in)
		assert.Equal(t, expected, value)
	}
	lb, ub := []byte("key0"), []byte("key9")
	err = mdb.Iterate(lb, ub, make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, len(vals), nIters)

	// iterate through only some
	nIters = 0
	lb, ub = []byte("key0"), []byte("key3")
	err = mdb.Iterate(lb, ub, make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, 2, nIters)

	// iterate through single value and send done signal
	nIters = 0
	done := make(chan struct{})
	callback = func(key, value []byte) {
		nIters++
		expected, in := vals[string(key)]
		assert.True(t, in)
		assert.Equal(t, expected, value)
		close(done)
	}
	lb, ub = []byte("key0"), []byte("key9")
	err = mdb.Iterate(lb, ub, done, callback)
	assert.Nil(t, err)
	assert.Equal(t, 1, nIters)
}
