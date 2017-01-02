package server

import (
	"container/heap"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/drausin/libri/libri/db"
	"github.com/stretchr/testify/assert"
)

func TestRoutingBucket_PushPop(t *testing.T) {
	for i := 2.0; i <= 8; i++ {
		nPeers := int(math.Pow(2, i))
		rb := newFirstBucket()
		for _, peer := range generatePeers(nPeers) {
			heap.Push(rb, peer)
		}
		prevPeer := heap.Pop(rb).(*Peer)
		for rb.Len() > 0 {
			curPeer := heap.Pop(rb).(*Peer)
			info := fmt.Sprintf("prev: %v, cur: %v", prevPeer.LatestResponse,
				curPeer.LatestResponse)
			assert.True(t, prevPeer.LatestResponse.Before(curPeer.LatestResponse), info)
			prevPeer = curPeer
		}
	}
}

func TestRoutingTable_SaveLoad(t *testing.T) {
	for s := 0; s < 8; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0).Rand(rng, IDUpperBound)

		for i := 0.0; i <= 7; i++ {
			nPeers := int(math.Pow(2, i))
			rt1, _, err := NewRoutingTableWithPeers(selfID, generatePeers(nPeers))
			assert.Nil(t, err)

			kvdb, err := db.NewTempDirRocksDB()
			assert.Nil(t, err)

			err = rt1.Save(kvdb)
			assert.Nil(t, err)

			rt2, err := LoadRoutingTable(kvdb)
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
	}
}

func TestRoutingTable_NumActivePeers(t *testing.T) {
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0).Rand(rng, IDUpperBound)

		// make sure handles zero peers
		rt, nAdded, err := NewRoutingTableWithPeers(selfID, generatePeers(0))
		assert.Nil(t, err)
		assert.Equal(t, 0, nAdded)
		assert.Equal(t, 0, rt.NumActivePeers())

		for i := 0.0; i <= 8; i++ {
			nPeers := int(math.Pow(2, i))
			info := fmt.Sprintf("s: %v, nPeers: %v", s, nPeers)
			rt, nAdded, err = NewRoutingTableWithPeers(selfID, generatePeers(nPeers))
			assert.Nil(t, err)
			assert.Equal(t, nAdded, rt.NumActivePeers(), info)
		}
	}
}

func TestRoutingTable_AddPeer(t *testing.T) {
	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		selfID := big.NewInt(0)
		selfID.Rand(rng, IDUpperBound)
		rt := NewEmptyRoutingTable(selfID)
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

	// make sure we error on k < 1
	rt, _, err := NewRoutingTableWithPeers(big.NewInt(0), generatePeers(8))
	assert.Nil(t, err)
	_, _, err = rt.PopNextPeers(big.NewInt(0), -1)
	assert.NotNil(t, err)
	_, _, err = rt.PopNextPeers(big.NewInt(0), 0)
	assert.NotNil(t, err)

	for nPeers := 8; nPeers <= 128; nPeers *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs
			rng := rand.New(rand.NewSource(int64(s)))
			selfID := big.NewInt(0).Rand(rng, IDUpperBound)
			rt, _, err := NewRoutingTableWithPeers(selfID, generatePeers(nPeers))
			assert.Nil(t, err)

			target := big.NewInt(0).Rand(rng, IDUpperBound)

			for k := 2; k <= 32; k *= 2 {
				// for different numbers of peers to get
				numActivePeers := rt.NumActivePeers()
				info := fmt.Sprintf("nPeers: %v, s: %v, k: %v, nap: %v", nPeers, s,
					k, numActivePeers)
				nextPeers, bucketIdxs, err := rt.PopNextPeers(target, k)
				checkNextPeers(t, rt, k, numActivePeers, nextPeers, bucketIdxs,
					err, info)

				// check that the number of active peers has decreased by number
				// of nextPeers
				assert.Equal(t, numActivePeers-len(nextPeers), rt.NumActivePeers())

				// check that no peer exists in our peers maps
				for i, nextPeer := range nextPeers {
					_, exists := rt.Peers[nextPeer.IDStr]
					assert.False(t, exists)
					_, exists =
						rt.Buckets[bucketIdxs[i]].Positions[nextPeer.IDStr]
					assert.False(t, exists)
				}
			}

		}
	}
}

func TestRoutingTable_PeakNextPeers(t *testing.T) {

	// make sure we error on k < 1
	rt, _, err := NewRoutingTableWithPeers(big.NewInt(0), generatePeers(8))
	assert.Nil(t, err)
	_, _, err = rt.PeakNextPeers(big.NewInt(0), -1)
	assert.NotNil(t, err)
	_, _, err = rt.PeakNextPeers(big.NewInt(0), 0)
	assert.NotNil(t, err)

	for nPeers := 8; nPeers <= 128; nPeers *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs
			rng := rand.New(rand.NewSource(int64(s)))
			selfID := big.NewInt(0).Rand(rng, IDUpperBound)
			rt, _, err := NewRoutingTableWithPeers(selfID, generatePeers(nPeers))
			assert.Nil(t, err)

			target := big.NewInt(0).Rand(rng, IDUpperBound)

			for k := 2; k <= 32; k *= 2 {
				// for different numbers of peers to get
				numActivePeers := rt.NumActivePeers()
				info := fmt.Sprintf("nPeers: %v, s: %v, k: %v, nap: %v", nPeers,
					s, k, numActivePeers)
				nextPeers, bucketIdxs, err := rt.PeakNextPeers(target, k)
				checkNextPeers(t, rt, k, numActivePeers, nextPeers, bucketIdxs,
					err, info)

				// check that the number of active peers remains unchanged
				assert.Equal(t, numActivePeers, rt.NumActivePeers())

				// check that every peer exists in our peers maps
				for i, nextPeer := range nextPeers {
					_, exists := rt.Peers[nextPeer.IDStr]
					assert.True(t, exists)
					_, exists =
						rt.Buckets[bucketIdxs[i]].Positions[nextPeer.IDStr]
					assert.True(t, exists)
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
		address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", 11000+p))
		if err != nil {
			panic(err)
		}
		peer, err := NewPeer(
			id,
			fmt.Sprintf("peer-%d", p),
			address,
			time.Unix(int64(p), 0).UTC(),
		)
		if err != nil {
			panic(err)
		}

		peers[peer.IDStr] = peer
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
			pID := rt.Buckets[b].ActivePeers[p].ID
			assert.True(t, pID.Cmp(lowerBound) >= 0)
			assert.True(t, pID.Cmp(upperBound) < 0)
			nPeers++
		}
	}
	assert.Equal(t, nExpectedPeers, nPeers)
	assert.Equal(t, 1, nContainSelf)
}

func newIntLsh(x int64, n uint) *big.Int {
	return big.NewInt(0).Lsh(big.NewInt(x), n)
}

func checkNextPeers(t *testing.T, rt *RoutingTable, k int, numActivePeers int, nextPeers []*Peer,
	bucketIdxs []int, err error, info string) {

	nextPeersSet := make(map[string]struct{})
	var s struct{}
	for i, nextPeer := range nextPeers {
		assert.NotNil(t, nextPeer)
		assert.True(t, rt.Buckets[bucketIdxs[i]].Contains(nextPeer.ID))

		// check peer is not yet in set
		_, exists := nextPeersSet[nextPeer.IDStr]
		assert.False(t, exists)

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
