package storage

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/db"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestKvdbStorerLoader_StoreLoad(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	cases := []struct {
		ns    []byte
		key   []byte
		value []byte
	}{
		{[]byte("ns"), bytes.Repeat([]byte{0}, cid.Length), []byte{0}},
		{[]byte("ns"), bytes.Repeat([]byte{0}, cid.Length),
			bytes.Repeat([]byte{255}, 1024)},
		{[]byte("test namespace"), cid.NewPseudoRandom(rng).Bytes(), []byte("test value")},
	}

	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	sl := NewKVDBStorerLoader(kvdb, NewMaxLengthChecker(256), NewMaxLengthChecker(1024))

	for _, c := range cases {
		err := sl.Store(c.ns, c.key, c.value)
		assert.Nil(t, err)

		loaded, err := sl.Load(c.ns, c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)
	}
}
