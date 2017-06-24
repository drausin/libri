package storage

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/db"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestKvdbSLD_StoreLoadDelete(t *testing.T) {
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

	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	sld := NewKVDBStorerLoaderDeleter(kvdb, NewMaxLengthChecker(256), NewMaxLengthChecker(1024))

	for _, c := range cases {
		err := sld.Store(c.ns, c.key, c.value)
		assert.Nil(t, err)

		loaded, err := sld.Load(c.ns, c.key)
		assert.Nil(t, err)
		assert.Equal(t, c.value, loaded)

		err = sld.Delete(c.ns, c.key)
		assert.Nil(t, err)
	}
}
