package server

import (
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestRoutingTable_PopNextPeers(t *testing.T) {

	setUpRoutingTable := func(selfID *big.Int, nPeers int) *RoutingTable {
		rt := &RoutingTable{
			SelfID:  selfID,
			Peers:   make(map[string]*Peer),
			Buckets: []*RoutingBucket{newFirstBucket()},
		}
		for _, peer := range generatePeers(nPeers) {
			rt.AddPeer(peer)
		}
		return rt
	}

	// make sure we error on k < 1
	rt := setUpRoutingTable(big.NewInt(0), 8)
	_, _, err := rt.PeakNextPeers(big.NewInt(0), -1)
	assert.NotNil(t, err)
	_, _, err = rt.PeakNextPeers(big.NewInt(0), 0)
	assert.NotNil(t, err)

	for nPeers := 8; nPeers <= 128; nPeers *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs
			rng := rand.New(rand.NewSource(int64(s)))
			selfID := big.NewInt(0).Rand(rng, IDUpperBound)
			rt := setUpRoutingTable(selfID, nPeers)

			target := big.NewInt(0).Rand(rng, IDUpperBound)

			for k := 2; k <= 32; k *= 2 {
				// for different numbers of peers to get
				numActivePeers := rt.NumActivePeers()
				info := fmt.Sprintf("nPeers: %v, s: %v, k: %v, nap: %v", nPeers, s, k, numActivePeers)
				nextPeers, bucketIdxs, err := rt.PopNextPeers(target, k)

				nextPeersSet := make(map[string]struct{})
				var s struct{}
				for i, nextPeer := range nextPeers {
					assert.NotNil(t, nextPeer)
					assert.True(t, rt.Buckets[bucketIdxs[i]].Contains(nextPeer.ID))

					// check peer is not yet in set
					_, ok := nextPeersSet[nextPeer.IDStr]
					assert.False(t, ok)

					// add this peer to the set
					nextPeersSet[nextPeer.IDStr] = s
				}

				if numActivePeers == 0 {
					// if there are no active peers, we should have gotten an error
					assert.NotNil(t, err, info)

				} else if k < numActivePeers {
					// should get k peers
					assert.Nil(t, err)
					assert.Equal(t, k, len(nextPeers), info)
					assert.Equal(t, k, len(bucketIdxs), info)

				} else {
					// should get numActivePeers
					assert.Nil(t, err)
					assert.Equal(t, numActivePeers, len(nextPeers), info)
					assert.Equal(t, numActivePeers, len(bucketIdxs), info)
				}
			}

		}
	}
}

func TestRoutingTable_chooseBucketIndex(t *testing.T) {

	rt := &RoutingTable{
		Buckets: []*RoutingBucket{
			{LowerBound: big.NewInt(0), UpperBound: big.NewInt(64)},
			{LowerBound: big.NewInt(64), UpperBound: big.NewInt(128)},
			{LowerBound: big.NewInt(128), UpperBound: big.NewInt(192)},
			{LowerBound: big.NewInt(192), UpperBound: big.NewInt(255)},
		},
	}
	var target *big.Int
	var nextIdx int
	var err error

	target = big.NewInt(150)

	// bucket 2 should be next since it contains target
	nextIdx, err = rt.chooseBucketIndex(target, 2, 1)
	assert.Nil(t, err)
	assert.Equal(t, 2, nextIdx)

	// bucket 3 should be next because it's upper bound (255) is closer to the target than 1's
	// lower bound (64) via XOR distance
	// 	XOR(150, 255)	= 10010110 ^ 11111111
	//			= 01101001
	//			= 105
	//
	//	XOR(150, 64)	= 10010110 ^ 01000000
	//			= 11010110
	//			= 214
	nextIdx, err = rt.chooseBucketIndex(target, 3, 1)
	assert.Nil(t, err)
	assert.Equal(t, 3, nextIdx)

	// bucket 1 should be next since 4 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 4, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, nextIdx)

	// bucket 0 should be next since 4 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 4, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, nextIdx)

	// should get error because forward and backward indices are out of bounds
	_, err = rt.chooseBucketIndex(target, 4, -1)
	assert.NotNil(t, err)

	target = big.NewInt(100)

	// bucket 1 should be next since it contains target
	nextIdx, err = rt.chooseBucketIndex(target, 1, 0)
	assert.Nil(t, err)
	assert.Equal(t, 1, nextIdx)

	// bucket 0 should be next since it's lower bound (0) is closer to the target than 2's
	// upper bound (192) via XOR distance
	//	XOR(100, 0)	= 01100100 ^ 00000000
	//			= 01100100
	//			= 100
	//
	//	XOR(100, 192)	= 01100100 ^ 11000000
	//			= 10100100
	//			= 164
	nextIdx, err = rt.chooseBucketIndex(target, 2, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, nextIdx)

	target = big.NewInt(50)

	// bucket 0 should be next since it contains target
	nextIdx, err = rt.chooseBucketIndex(target, 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, 0, nextIdx)

	// bucket 1 should be next since -1 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 1, -1)
	assert.Nil(t, err)
	assert.Equal(t, 1, nextIdx)

	// bucket 2 should be next since -1 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 2, -1)
	assert.Nil(t, err)
	assert.Equal(t, 2, nextIdx)

	// bucket 3 should be next since -1 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 3, -1)
	assert.Nil(t, err)
	assert.Equal(t, 3, nextIdx)

	// should get error because forward and backward indices are out of bounds
	_, err = rt.chooseBucketIndex(target, 4, -1)
	assert.NotNil(t, err)

	target = big.NewInt(200)

	// bucket 3 should be next since it contains target
	nextIdx, err = rt.chooseBucketIndex(target, 3, 2)
	assert.Nil(t, err)
	assert.Equal(t, 3, nextIdx)

	// bucket 2 should be next since 4 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 4, 2)
	assert.Nil(t, err)
	assert.Equal(t, 2, nextIdx)

	// bucket 1 should be next since 4 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 4, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, nextIdx)

	// bucket 0 should be next since 4 is out of bounds
	nextIdx, err = rt.chooseBucketIndex(target, 4, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, nextIdx)

	// should get error because forward and backward indices are out of bounds
	_, err = rt.chooseBucketIndex(target, 4, -1)
	assert.NotNil(t, err)

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
	for s := 0; s < 16; s++ {
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
	for s := 0; s < 16; s++ {
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

	check(big.NewInt(0), 0, newIntLsh(1, 255))                       // no prefix
	check(newIntLsh(128, 248), 1, newIntLsh(192, 248))               // prefix 1
	check(newIntLsh(1, 254), 2, newIntLsh(3, 253))                   // prefix 01
	check(newIntLsh(3, 254), 2, newIntLsh(7, 253))                   // prefix 11
	check(newIntLsh(170, 248), 7, newIntLsh(171, 248))               // prefix 1010101
	check(newIntLsh(128, 248), 7, newIntLsh(129, 248))               // prefix 1000000
	check(newIntLsh(254, 248), 7, newIntLsh(255, 248))               // prefix 1111111
	check(newIntLsh(0, 248), 8, newIntLsh(128, 240))                 // prefix 00000000
	check(newIntLsh(128, 248), 8, newIntLsh(128<<8|128, 240))        // prefix 10000000
	check(newIntLsh(255, 248), 8, newIntLsh(255<<8|128, 240))        // prefix 11111111
	check(newIntLsh(0, 240), 9, newIntLsh(64, 240))                  // prefix 00000000 0
	check(newIntLsh(128, 240), 9, newIntLsh(192, 240))               // prefix 00000000 1
	check(newIntLsh(255, 248), 9, newIntLsh(255<<8|64, 240))         // prefix 11111111 0
	check(newIntLsh(255<<8|128, 240), 9, newIntLsh(255<<8|192, 240)) // prefix 11111111 1
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
			assert.True(t, pId.Cmp(lowerBound) >= 0)
			assert.True(t, pId.Cmp(upperBound) < 0)
			nPeers++
		}
	}
	assert.Equal(t, nExpectedPeers, nPeers)
	assert.Equal(t, 1, nContainSelf)
}

func newIntLsh(x int64, n uint) *big.Int {
	return big.NewInt(0).Lsh(big.NewInt(x), n)
}
