package storage

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestKvdbSLD_StoreLoadDelete(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	cases := []struct {
		ns    []byte
		key   []byte
		value []byte
	}{
		{[]byte("ns"), bytes.Repeat([]byte{0}, id.Length), []byte{0}},
		{[]byte("ns"), bytes.Repeat([]byte{0}, id.Length),
			bytes.Repeat([]byte{255}, 1024)},
		{[]byte("test namespace"), id.NewPseudoRandom(rng).Bytes(), []byte("test value")},
	}

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)

	for _, c := range cases {
		sld := NewKVDBStorerLoaderDeleter(
			c.ns,
			kvdb,
			NewMaxLengthChecker(256),
			NewMaxLengthChecker(1024),
		)
		err := sld.Store(c.key, c.value)
		assert.Nil(t, err)

		loaded, err := sld.Load(c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)

		err = sld.Delete(c.key)
		assert.Nil(t, err)
	}
}

func TestKvdbSLD_Iterate(t *testing.T) {
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)

	ns := []byte("key")
	sld := NewKVDBStorerLoaderDeleter(ns, kvdb, NewMaxLengthChecker(256), NewMaxLengthChecker(1024))

	vals := map[string][]byte{
		"1": []byte("val1"),
		"2": []byte("val2"),
		"3": []byte("val3"),
	}
	for key, val := range vals {
		err := sld.Store([]byte(key), val)
		assert.Nil(t, err)
	}

	nIters := 0
	callback := func(key, value []byte) {
		nIters++
		expected, in := vals[string(key)]
		assert.True(t, in)
		assert.Equal(t, expected, value)
	}
	lb, ub := []byte("0"), []byte("9")
	err = sld.Iterate(lb, ub, make(chan struct{}), callback)
	assert.Nil(t, err)
	assert.Equal(t, len(vals), nIters)
}
