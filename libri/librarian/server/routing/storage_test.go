package routing

import (
	"container/heap"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestFromStored(t *testing.T) {
	srt := newTestStoredTable(rand.New(rand.NewSource(0)), 128)
	rt := fromStored(srt)
	assertRoutingTablesEqual(t, rt, srt)
}

func TestToRoutingTable(t *testing.T) {
	rt, _, _ := NewTestWithPeers(rand.New(rand.NewSource(0)), 128)
	srt := toStored(rt)
	assertRoutingTablesEqual(t, rt, srt)
}

func TestRoutingTable_SaveLoad(t *testing.T) {
	rt1, _, _ := NewTestWithPeers(rand.New(rand.NewSource(0)), 8)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	ssl := storage.NewServerKVDBStorerLoader(kvdb)

	err = rt1.Save(ssl)
	assert.Nil(t, err)

	rt2, err := Load(ssl)
	assert.Nil(t, err)

	// check that routing tables are the same
	assert.Equal(t, rt1.SelfID().Bytes(), rt2.SelfID().Bytes())
	assert.Equal(t, len(rt1.(*table).peers), len(rt2.(*table).peers))
	assert.Equal(t, rt1.(*table).NumPeers(), rt2.(*table).NumPeers())
	assert.Equal(t, rt1.(*table).peers, rt2.(*table).peers)

	// descend into the protected table fields to check equality
	for bi, bucket1 := range rt1.(*table).buckets {
		bucket2 := rt2.(*table).buckets[bi]
		assert.Equal(t, bucket1.depth, bucket2.depth)
		assert.Equal(t, bucket1.lowerBound, bucket2.lowerBound)
		assert.Equal(t, bucket1.upperBound, bucket2.upperBound)
		assert.Equal(t, bucket1.containsSelf, bucket2.containsSelf)
		assert.Equal(t, bucket1.Len(), bucket2.Len())

		// the ActivePeers array may have some small differences in
		// ordering (and thus the Position map will also reflect those
		// differences), but here we check that the same peers are popped
		// off at the same time, which is really want we care about
		for bucket2.Len() > 0 {
			peer1, peer2 := heap.Pop(bucket1), heap.Pop(bucket2)
			assert.Equal(t, peer1, peer2)
		}
	}
}

func newTestStoredTable(rng *rand.Rand, n int) *storage.RoutingTable {
	rt := &storage.RoutingTable{
		SelfId: cid.NewPseudoRandom(rng).Bytes(),
		Peers:  make([]*storage.Peer, n),
	}
	for i := 0; i < n; i++ {
		rt.Peers[i] = peer.NewTestStoredPeer(rng, i)
	}
	return rt
}

func assertRoutingTablesEqual(t *testing.T, rt Table, srt *storage.RoutingTable) {
	assert.Equal(t, srt.SelfId, rt.SelfID().Bytes())
	for _, sp := range srt.Peers {
		spIDStr := cid.FromBytes(sp.Id).String()
		if toPeer, exists := rt.(*table).peers[spIDStr]; exists {
			peer.AssertPeersEqual(t, sp, toPeer)
		}
	}
}

type fixedLoader struct {
	bytes []byte
	err   error
}

func (l *fixedLoader) Load(key []byte) ([]byte, error) {
	return l.bytes, l.err
}

func TestLoad_err(t *testing.T) {

	// simulates missing/not stored table
	rt1, err := Load(&fixedLoader{
		bytes: nil,
		err:   nil,
	})
	assert.Nil(t, rt1)
	assert.Nil(t, err)

	// simulates loading error
	rt2, err := Load(&fixedLoader{
		bytes: []byte("some random bytes"),
		err:   errors.New("some random error"),
	})
	assert.Nil(t, rt2)
	assert.NotNil(t, err)

	// simulates bad stored table
	rt3, err := Load(&fixedLoader{
		bytes: []byte("the wrong bytes"),
		err:   nil,
	})
	assert.Nil(t, rt3)
	assert.NotNil(t, err)
}
