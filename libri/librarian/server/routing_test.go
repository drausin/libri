package server

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

/*
func TestRoutingTable_NextPeers(t *testing.T) {
	setUpRoutingTable := func(selfID []byte) *RoutingTable {
		peers := generatePeers(128)
		rt := &RoutingTable{
			SelfID:  selfID,
			Peers:   make(map[string]*Peer),
			Buckets: []*RoutingBucket{newFirstBucket()},
		}
		for _, peer := range peers {
			rt.AddPeer(peer)
		}
		return rt
	}

	for s := 0; s < 16; s++ {
		selfID := sha256.Sum256([]byte{byte(s)})
		rt := setUpRoutingTable(selfID[:])
		for n := 1; n < 20; n++ {
			target := sha256.Sum256([]byte{byte(n)})
			 nextPeers, bucketIdxs := rt.NextPeers(target[:], n)
		}
	}
}
*/

func TestRoutingTable_AddPeer(t *testing.T) {
	setUpRoutingTable := func(selfID *big.Int) *RoutingTable {
		return &RoutingTable{
			SelfID:  selfID,
			Peers:   make(map[string]*Peer),
			Buckets: []*RoutingBucket{newFirstBucket()},
		}
	}

	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0)
		selfID.Rand(rng, IDUpperBound)
		rt := setUpRoutingTable(selfID)
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
				if rng.Float32() < 0.25 {
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

func TestRoutingTable_SplitBucket(t *testing.T) {
	setUpRoutingTable := func(selfID *big.Int) *RoutingTable {
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
	for s := 0; s < 1; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0).Rand(rng, IDUpperBound)
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

	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 1; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0).Rand(rng, IDUpperBound)
		rt := setUpRoutingTable(selfID)

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
	check := func(lowerBound *big.Int, depth uint, expected *big.Int) {
		actual := splitLowerBound(lowerBound, depth)
		assert.Equal(t, expected, actual)
	}

	NewIntLsh := func(x int64, n uint) *big.Int {
		return big.NewInt(0).Lsh(big.NewInt(x), n)
	}

	check(big.NewInt(0), 0, NewIntLsh(1, 255))                       // no prefix
	check(NewIntLsh(128, 248), 1, NewIntLsh(192, 248))               // prefix 1
	check(NewIntLsh(1, 254), 2, NewIntLsh(3, 253))                   // prefix 01
	check(NewIntLsh(3, 254), 2, NewIntLsh(7, 253))                   // prefix 11
	check(NewIntLsh(170, 248), 7, NewIntLsh(171, 248))               // prefix 1010101
	check(NewIntLsh(128, 248), 7, NewIntLsh(129, 248))               // prefix 1000000
	check(NewIntLsh(254, 248), 7, NewIntLsh(255, 248))               // prefix 1111111
	check(NewIntLsh(0, 248), 8, NewIntLsh(128, 240))                 // prefix 00000000
	check(NewIntLsh(128, 248), 8, NewIntLsh(128<<8|128, 240))        // prefix 10000000
	check(NewIntLsh(255, 248), 8, NewIntLsh(255<<8|128, 240))        // prefix 11111111
	check(NewIntLsh(0, 240), 9, NewIntLsh(64, 240))                  // prefix 00000000 0
	check(NewIntLsh(128, 240), 9, NewIntLsh(192, 240))               // prefix 00000000 1
	check(NewIntLsh(255, 248), 9, NewIntLsh(255<<8|64, 240))         // prefix 11111111 0
	check(NewIntLsh(255<<8|128, 240), 9, NewIntLsh(255<<8|192, 240)) // prefix 11111111 1
}

func generatePeers(nPeers int) map[string]*Peer {
	rng := rand.New(rand.NewSource(int64(nPeers)))
	peers := make(map[string]*Peer)
	for p := 0; p < nPeers; p++ {
		id := big.NewInt(0)
		id.Rand(rng, IDUpperBound)
		peer, err := NewPeer(
			id,
			fmt.Sprintf("peer-%d", p),
			nil, // we don't need the host into for these tests
			time.Unix(int64(p), 0).UTC(),
		)
		if err != nil {
			panic(err)
		}

		peers[peer.IDStr] = &peer
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
			pId := rt.Buckets[b].ActivePeers[p].ID
			log.Printf("pId: %v, lowerBound: %s", pId, lowerBound)
			assert.True(t, pId.Cmp(lowerBound) >= 0)
			assert.True(t, pId.Cmp(upperBound) < 0)
			nPeers++
		}
	}
	assert.Equal(t, nExpectedPeers, nPeers)
	assert.Equal(t, 1, nContainSelf)
}
