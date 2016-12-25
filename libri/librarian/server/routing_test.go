package server

import (
	"bytes"
	"crypto/sha256"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddPeer(t *testing.T) {
	setUpRoutingTable := func(selfID []byte) *RoutingTable {
		return &RoutingTable{
			SelfID:  selfID,
			Peers:   make(map[string]*Peer),
			Buckets: []*RoutingBucket{newFirstBucket()},
		}
	}

	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 1; s++ {
		selfID := sha256.Sum256([]byte{byte(s)})
		rt := setUpRoutingTable(selfID[:])
		nPeers := 0
		peers := generatePeers(100)
		repeatedPeers := make([]*Peer, 0)
		for _, peer := range peers {
			bucketIdx, alreadyPresent, err := rt.AddPeer(peer)
			assert.Nil(t, err)
			assert.False(t, alreadyPresent)
			if bucketIdx != -1 {
				nPeers++
				// 25% of the time, we add this peer to list we'll add again later
				if peer.PeerId[0]%8 < 2 {
					repeatedPeers = append(repeatedPeers, peer)
				}
			}
			checkPeers(t, rt, nPeers)
		}

		// add a few other peers a second time
		for _, peer := range repeatedPeers {
			_, alreadyPresent, err := rt.AddPeer(peer)
			assert.Nil(t, err)
			assert.True(t, alreadyPresent)
			checkPeers(t, rt, nPeers)
		}
	}

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
		checkPeers(t, rt, len(rt.Peers))
	}

	// try same split sequence with different selfIDs
	for s := 0; s < 10; s++ {
		selfID := sha256.Sum256([]byte{byte(s)})
		rt := setUpRoutingTable(selfID[:])

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

	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := sha256.Sum256([]byte{byte(s)})
		rt := setUpRoutingTable(selfID[:])

		// do pseudo-random splits
		for i := 0; i < 10; i++ {
			splitIdx := int(rng.Uint32()) % len(rt.Buckets)
			err := rt.splitBucket(splitIdx)
			assert.Nil(t, err)
			checkBuckets(rt, i+2)
		}
	}
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

func checkPeers(t *testing.T, rt *RoutingTable, nExpectedPeers int) {
	nContainSelf := 0
	nPeers := 0
	assert.True(t, sort.IsSorted(rt))
	for b := 0; b < len(rt.Buckets); b++ {
		lowerBound, upperBound := rt.Buckets[b].LowerBound, rt.Buckets[b].UpperBound
		if b > 0 {
			assert.Equal(t, lowerBound, rt.Buckets[b-1].UpperBound)
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
			nPeers++
		}
	}
	assert.Equal(t, nExpectedPeers, nPeers)
	assert.Equal(t, 1, nContainSelf)
}
