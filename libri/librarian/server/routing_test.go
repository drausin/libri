package server

import (
	"bytes"
	"crypto/sha256"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func generatePeers(nPeers int) map[string]*Peer {
	peers := make(map[string]*Peer)
	for p := 0; p < nPeers; p++ {
		peerIDa := sha256.Sum256([]byte{byte(p)}) // deterministic pseudo-randomness
		peerID := peerIDa[:]
		peerIDStr := IDString(peerID)

		// we don't need the host into for these tests, so just populate ID fields
		peers[peerIDStr] = &Peer{
			PeerId:    peerID,
			PeerIdStr: peerIDStr,
		}
	}
	return peers
}

func TestSplitBucket(t *testing.T) {
	setUpRoutingTable := func(selfID []byte) *RoutingTable {
		firstBucket := newFirstBucket()
		peers := generatePeers(firstBucket.MaxActivePeers)
		for _, peer := range peers {
			firstBucket.Push(peer)
		}
		return &RoutingTable{
			SelfID:  selfID,
			Peers:   peers,
			Buckets: []*RoutingBucket{firstBucket},
		}
	}

	checkBuckets := func(rt *RoutingTable, nExpectedBuckets int) {
		assert.Equal(t, nExpectedBuckets, len(rt.Buckets))
		assert.True(t, sort.IsSorted(rt))

		// check peers obey the bounds for their buckets
		nContainSelf := 0
		for b := 0; b < len(rt.Buckets); b++ {
			lowerBound, upperBound := rt.Buckets[b].LowerBound, rt.Buckets[b].UpperBound
			if b > 0 {
				assert.Equal(t, lowerBound, rt.Buckets[b-1].UpperBound,
					"b: %v @depth %v, b-1: %v @depth %v", b,
					rt.Buckets[b].Depth, b-1, rt.Buckets[b-1].Depth)
			}
			if rt.Buckets[b].ContainsSelf {
				assert.True(t, rt.Buckets[b].Contains(rt.SelfID))
				nContainSelf++
			} else {
				assert.False(t, rt.Buckets[b].Contains(rt.SelfID))
			}
			for p := range rt.Buckets[b].ActivePeers {
				pId := rt.Buckets[b].ActivePeers[p].PeerId
				assert.True(t, bytes.Compare(pId, lowerBound) >= 0)
				assert.True(t, bytes.Compare(pId, upperBound) < 0)
			}
		}
		assert.Equal(t, 1, nContainSelf)
	}

	selfID := bytes.Repeat([]byte{1}, NodeIDLength) // fixed but arbitrary
	rt := setUpRoutingTable(selfID)

	// lower bounds: [0 1]
	err := rt.splitBucket(0)
	assert.Nil(t, err)
	checkBuckets(rt, 2)

	// lower bounds: [0 10 11]
	err = rt.splitBucket(1)
	assert.Nil(t, err)
	checkBuckets(rt, 3)

	// lower bounds: [0 10 110 111]
	err = rt.splitBucket(2)
	assert.Nil(t, err)
	checkBuckets(rt, 4)

	// lower bounds: [0 100 101 110 111]
	err = rt.splitBucket(1)
	assert.Nil(t, err)
	checkBuckets(rt, 5)

	// lower bounds: [00 01 100 101 110 111]
	err = rt.splitBucket(0)
	assert.Nil(t, err)
	checkBuckets(rt, 6)
}

func TestSplitLowerBound_Ok(t *testing.T) {
	check := func(lowerBound []byte, depth uint, expected []byte) {
		actual, err := splitLowerBound(lowerBound, depth)
		assert.Nil(t, err)
		assert.Equal(t, expected, actual)
	}

	check([]byte{0}, 0, []byte{1 << 7})          // no prefix
	check([]byte{1 << 7}, 1, []byte{3 << 6})     // prefix 1
	check([]byte{1 << 6}, 2, []byte{3 << 5})     // prefix 01
	check([]byte{3 << 6}, 2, []byte{7 << 5})     // prefix 11
	check([]byte{85 << 1}, 2, []byte{170})       // prefix 1010101
	check([]byte{128}, 7, []byte{129})           // prefix 1000000
	check([]byte{254}, 7, []byte{255})           // prefix 1111111
	check([]byte{0, 0}, 8, []byte{0, 128})       // prefix 00000000
	check([]byte{128, 0}, 8, []byte{128, 128})   // prefix 10000000
	check([]byte{255, 0}, 8, []byte{255, 128})   // prefix 11111111
	check([]byte{0, 0}, 9, []byte{0, 64})        // prefix 00000000 0
	check([]byte{0, 128}, 9, []byte{0, 192})     // prefix 00000000 1
	check([]byte{255, 0}, 9, []byte{255, 64})    // prefix 11111111 0
	check([]byte{255, 128}, 9, []byte{255, 192}) // prefix 11111111 1
}

func TestSplitLowerBound_Error(t *testing.T) {
	check := func(lowerBound []byte, depth uint) {
		_, err := splitLowerBound(lowerBound, depth)
		assert.NotNil(t, err)
	}

	// should error b/c not enough room in byte array
	check([]byte{}, 0)
	check([]byte{}, 1)
	check([]byte{}, 2)
	check([]byte{0}, 8)
	check([]byte{0}, 9)
	check([]byte{0, 0}, 16)
	check([]byte{0, 0}, 17)
}
