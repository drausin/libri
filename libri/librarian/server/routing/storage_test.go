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

	err = Save(kvdb, rt1)
	assert.Nil(t, err)

	rt2, err := Load(kvdb)
	assert.Nil(t, err)

	// check that routing tables are the same
	assert.Equal(t, rt1.SelfID, rt2.SelfID)
	assert.Equal(t, rt1.Peers, rt2.Peers)
	assert.Equal(t, rt1.Len(), rt2.Len())
	for bi, bucket1 := range rt1.Buckets {
		bucket2 := rt2.Buckets[bi]
		assert.Equal(t, bucket1.Depth, bucket2.Depth)
		assert.Equal(t, bucket1.LowerBound, bucket2.LowerBound)
		assert.Equal(t, bucket1.UpperBound, bucket2.UpperBound)
		assert.Equal(t, bucket1.ContainsSelf, bucket2.ContainsSelf)
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

	kvdb.Close()
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

func assertRoutingTablesEqual(t *testing.T, rt *Table, srt *storage.RoutingTable) {
	assert.Equal(t, srt.SelfId, rt.SelfID.Bytes())
	for _, sp := range srt.Peers {
		spIDStr := cid.String(cid.FromBytes(sp.Id))
		if toPeer, exists := rt.Peers[spIDStr]; exists {
			peer.AssertPeersEqual(t, sp, toPeer)
		}
	}
}
