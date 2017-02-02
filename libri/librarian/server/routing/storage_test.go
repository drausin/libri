package routing

import (
	"container/heap"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/stretchr/testify/assert"
)

func TestFromStored(t *testing.T) {
	srt := newTestStoredTable(rand.New(rand.NewSource(0)), 128)
	rt := fromStored(srt)
	assertRoutingTablesEqual(t, rt, srt)
}

func TestToRoutingTable(t *testing.T) {
	rt := NewTestWithPeers(rand.New(rand.NewSource(0)), 128)
	srt := toStored(rt)
	assertRoutingTablesEqual(t, rt, srt)
}

func TestRoutingTable_SaveLoad(t *testing.T) {
	rt1 := NewTestWithPeers(rand.New(rand.NewSource(0)), 8)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer kvdb.Close()
	ssl := storage.NewServerStorerLoader(storage.NewKVDBStorerLoader(kvdb))

	err = rt1.Save(ssl)
	assert.Nil(t, err)

	rt2, err := Load(ssl)
	assert.Nil(t, err)

	// check that routing tables are the same
	assert.Equal(t, rt1.SelfID(), rt2.SelfID())
	assert.Equal(t, len(rt1.Peers()), len(rt2.Peers()))
	assert.Equal(t, rt1.NumPeers(), rt2.NumPeers())
	assert.Equal(t, rt1.Peers(), rt2.Peers())

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
		if toPeer, exists := rt.Peers()[spIDStr]; exists {
			peer.AssertPeersEqual(t, sp, toPeer)
		}
	}
}
