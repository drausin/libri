package routing

import (
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestTable_NewWithPeers(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	for n := 1; n <= 256; n *= 2 {
		rt, _, nAdded := NewTestWithPeers(rng, n)
		assert.Equal(t, len(rt.(*table).peers), nAdded)
		assert.Equal(t, len(rt.(*table).peers), int(rt.(*table).NumPeers()))
	}
}

func TestTable_NewWithPeers_concurrent(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	concurrency := 4
	for n := concurrency; n <= 256; n *= 2 {
		rt := NewEmpty(cid.NewPseudoRandom(rng), NewDefaultParameters())
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(i int, wg *sync.WaitGroup) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(int64(i + 1)))
				for _, p := range peer.NewTestPeers(rng, n/concurrency) {
					rt.Push(p)
				}
			}(i, &wg)
		}

		wg.Wait()
		assert.Equal(t, len(rt.(*table).peers), int(rt.(*table).NumPeers()))
	}
}

func TestTable_NumPeers(t *testing.T) {
	for s := 0; s < 16; s++ {
		// make sure handles zero peers
		rng := rand.New(rand.NewSource(int64(s)))
		rt, _, _ := NewTestWithPeers(rng, 0)
		assert.Equal(t, 0, int(rt.(*table).NumPeers()))

		for n := 1; n <= 256; n *= 2 {
			info := fmt.Sprintf("s: %v, n: %v", s, n)
			rt, _, _ = NewTestWithPeers(rng, n)
			assert.Equal(t, len(rt.(*table).peers), int(rt.(*table).NumPeers()), info)
		}
	}
}

func TestTable_NumBuckets(t *testing.T) {
	// make sure handles single bucket case
	rng := rand.New(rand.NewSource(0))
	rt, _, _ := NewTestWithPeers(rng, 0)
	assert.Equal(t, 1, int(rt.(*table).NumBuckets()))

	for n := 1; n <= 256; n *= 2 {
		rt, _, _ = NewTestWithPeers(rng, n)
		assert.Equal(t, len(rt.(*table).buckets), rt.(*table).NumBuckets())
	}
}

func TestTable_Push(t *testing.T) {
	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		rt, _, _ := NewTestWithPeers(rng, 0) // empty
		peers := peer.NewTestPeers(rng, 128)
		repeatedPeers := make([]peer.Peer, 0)
		c := 0
		for _, p := range peers {
			status := rt.Push(p)
			assert.True(t, status == Added || status == Dropped)
			if status == Added {
				c++
				// 25% of the time, we add this peer to list we'll add again later
				if rng.Float32() < 0.25 {
					repeatedPeers = append(repeatedPeers, p)
				}
			}
			checkTableConsistent(t, rt, c)
		}

		// add a few other peers a second time
		for _, p := range repeatedPeers {
			status := rt.Push(p)
			assert.Equal(t, Existed, status)
			checkTableConsistent(t, rt, c)
		}
	}
}

func TestTable_Push_existing(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		rt, _, _ := NewTestWithPeers(rng, 128)

		// pop off a random peer
		ps := rt.Pop(cid.NewPseudoRandom(rng), 1)
		p1 := ps[0]
		qOutcomes1 := p1.Recorder().ToStored()

		// add it back
		assert.Equal(t, Added, rt.Push(p1))

		// check that recorder states are the same
		qOutcomes2 := rt.Get(p1.ID()).Recorder().ToStored()
		assert.Equal(t, qOutcomes1, qOutcomes2)

		// add it again, confirming that the recorder has the same state (i.e., no merge)
		assert.Equal(t, Existed, rt.Push(p1))
		qOutcomes3 := rt.Get(p1.ID()).Recorder().ToStored()
		assert.Equal(t, qOutcomes1, qOutcomes3)

		// create stub peer with new request success
		p2 := peer.NewStub(p1.ID(), peer.MissingName)
		p2.Recorder().Record(peer.Request, peer.Success)

		// add the stub, which should merge with the existing peer
		assert.Equal(t, Existed, rt.Push(p2))
		qOutcomes4 := rt.Get(p1.ID()).Recorder().ToStored()
		assert.NotEqual(t, qOutcomes1, qOutcomes4)
		assert.Equal(t, uint64(0), qOutcomes1.Requests.NQueries)
		assert.Equal(t, uint64(1), qOutcomes4.Requests.NQueries)
	}
}

func TestTable_Pop(t *testing.T) {

	// make sure we support poppping 0 peers
	rng := rand.New(rand.NewSource(0))
	rt, _, _ := NewTestWithPeers(rng, 8)
	ps := rt.Pop(cid.NewPseudoRandom(rng), 0)
	assert.Equal(t, 0, len(ps))

	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 8; s++ {
			// for different selfIDs
			rng := rand.New(rand.NewSource(int64(s)))
			rt, _, _ := NewTestWithPeers(rng, n)
			target := cid.NewPseudoRandom(rng)

			for k := uint(2); k <= 32; k *= 2 {
				// for different numbers of peers to get
				numActivePeers := rt.(*table).NumPeers()
				info := fmt.Sprintf("nPeers: %v, s: %v, k: %v, nap: %v", n, s,
					k, numActivePeers)
				ps = rt.Pop(target, k)
				checkPoppedPeers(t, k, numActivePeers, ps, info)

				// check that the number of active peers has decreased by len(ps)
				assert.Equal(t, numActivePeers-len(ps), rt.(*table).NumPeers())

				// check that no peer exists in our peers maps
				for _, nextPeer := range ps {
					_, exists := rt.(*table).peers[nextPeer.ID().String()]
					assert.False(t, exists)
				}
			}

		}
	}
}

func TestTable_Peak(t *testing.T) {

	// make sure we support poppping 0 peers
	rng := rand.New(rand.NewSource(0))
	rt, _, _ := NewTestWithPeers(rng, 8)
	ps := rt.Peak(cid.NewPseudoRandom(rng), 0)
	assert.Equal(t, 0, len(ps))

	for n := 8; n <= 16; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 8; s++ {
			// for different selfIDs
			rng := rand.New(rand.NewSource(int64(s)))
			rt, _, _ := NewTestWithPeers(rng, n)
			target := cid.NewPseudoRandom(rng)

			for k := uint(2); k <= 32; k *= 2 {
				// for different numbers of peers to get
				numActivePeers := rt.(*table).NumPeers()
				info := fmt.Sprintf("nPeers: %v, s: %v, k: %v, nap: %v", n, s,
					k, numActivePeers)
				ps = rt.Peak(target, k)
				checkPoppedPeers(t, k, numActivePeers, ps, info)

				// check that the number of active peers has not decreased
				assert.Equal(t, int(numActivePeers), int(rt.(*table).NumPeers()), info)
				assert.Equal(t, int(numActivePeers), len(rt.(*table).peers), info)

				// check that peers exist in our peers maps
				for _, nextPeer := range ps {
					_, exists := rt.(*table).peers[nextPeer.ID().String()]
					assert.True(t, exists)
				}
			}

		}
	}
}

func TestTable_Peak_concurrent(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, _, _ := NewTestWithPeers(rng, 256)
	target := cid.NewPseudoRandom(rng)
	concurrency := uint(4)

	for k := concurrency; k <= 32; k *= 2 {
		// for different numbers of peers to get
		numActivePeers := rt.(*table).NumPeers()
		var wg sync.WaitGroup
		for i := uint(0); i < concurrency; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				info := fmt.Sprintf("k: %v, nap: %v", k, numActivePeers)
				ps := rt.Peak(target, k/concurrency)
				checkPoppedPeers(t, k/concurrency, numActivePeers, ps, info)
			}(&wg)
		}
		wg.Wait()

		// check that the number of active peers has not decreased
		assert.Equal(t, int(numActivePeers), int(rt.(*table).NumPeers()))
		assert.Equal(t, int(numActivePeers), len(rt.(*table).peers))
	}
}

func TestTable_Sample(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for n := 2; n <= 256; n *= 2 {
		rt, _, _ := NewTestWithPeers(rng, n)
		for k := uint(2); k <= 32; k *= 2 {
			info := fmt.Sprintf("n: %v, k: %v", n, k)
			sample := rt.Sample(k, rng)
			if k <= uint(n) {
				assert.Equal(t, int(k), len(sample), info)
			} else {
				assert.Equal(t, n, len(sample), info)
			}
		}
	}
}

func TestTable_Less(t *testing.T) {
	rt := newSimpleTable()
	for i := 1; i < len(rt.buckets); i++ {
		assert.True(t, rt.Less(i-1, i))
		assert.False(t, rt.Less(i, i-1))
	}
}

func TestTable_Sort(t *testing.T) {
	// tests Len, Less, & Swap all together
	rt := newSimpleTable()
	rt.buckets[0], rt.buckets[2] = rt.buckets[2], rt.buckets[0]
	rt.buckets[1], rt.buckets[3] = rt.buckets[3], rt.buckets[1]

	// check it's current not sorted
	assert.False(t, rt.buckets[0].Before(rt.buckets[2]))
	assert.False(t, rt.buckets[1].Before(rt.buckets[3]))

	sort.Sort(rt)

	// check it's now sorted
	for i := 1; i < len(rt.buckets); i++ {
		assert.True(t, rt.buckets[i-1].Before(rt.buckets[i]))
	}
}

func TestTable_chooseBucketIndex(t *testing.T) {
	rt := newSimpleTable()

	target := cid.FromInt64(150)

	// bucket 2 should be next since it contains target
	i := rt.chooseBucketIndex(target, 2, 1)
	assert.Equal(t, 2, i)

	// bucket 3 should be next because it's upper bound (255) is closer to the target than 1's
	// lower bound (64) via XOR distance
	// 	XOR(150, 255)	= 10010110 ^ 11111111
	//			= 01101001
	//			= 105
	//
	//	XOR(150, 64)	= 10010110 ^ 01000000
	//			= 11010110
	//			= 214
	i = rt.chooseBucketIndex(target, 3, 1)
	assert.Equal(t, 3, i)

	// bucket 1 should be next since 4 is out of bounds
	i = rt.chooseBucketIndex(target, 4, 1)
	assert.Equal(t, 1, i)

	// bucket 0 should be next since 4 is out of bounds
	i = rt.chooseBucketIndex(target, 4, 0)
	assert.Equal(t, 0, i)

	// should panic because forward and backward indices are out of bounds
	assert.Panics(t, func() {
		rt.chooseBucketIndex(target, 4, -1)
	})

	target = cid.FromInt64(100)

	// bucket 1 should be next since it contains target
	i = rt.chooseBucketIndex(target, 1, 0)
	assert.Equal(t, 1, i)

	// bucket 0 should be next since it's lower bound (0) is closer to the target than 2's
	// upper bound (192) via XOR distance
	//	XOR(100, 0)	= 01100100 ^ 00000000
	//			= 01100100
	//			= 100
	//
	//	XOR(100, 192)	= 01100100 ^ 11000000
	//			= 10100100
	//			= 164
	i = rt.chooseBucketIndex(target, 2, 0)
	assert.Equal(t, 0, i)

	target = cid.FromInt64(50)

	// bucket 0 should be next since it contains target
	i = rt.chooseBucketIndex(target, 0, -1)
	assert.Equal(t, 0, i)

	// bucket 1 should be next since -1 is out of bounds
	i = rt.chooseBucketIndex(target, 1, -1)
	assert.Equal(t, 1, i)

	// bucket 2 should be next since -1 is out of bounds
	i = rt.chooseBucketIndex(target, 2, -1)
	assert.Equal(t, 2, i)

	// bucket 3 should be next since -1 is out of bounds
	i = rt.chooseBucketIndex(target, 3, -1)
	assert.Equal(t, 3, i)

	// should panic because forward and backward indices are out of bounds
	assert.Panics(t, func() {
		rt.chooseBucketIndex(target, 4, -1)
	})

	target = cid.FromInt64(200)

	// bucket 3 should be next since it contains target
	i = rt.chooseBucketIndex(target, 3, 2)
	assert.Equal(t, 3, i)

	// bucket 2 should be next since 4 is out of bounds
	i = rt.chooseBucketIndex(target, 4, 2)
	assert.Equal(t, 2, i)

	// bucket 1 should be next since 4 is out of bounds
	i = rt.chooseBucketIndex(target, 4, 1)
	assert.Equal(t, 1, i)

	// bucket 0 should be next since 4 is out of bounds
	i = rt.chooseBucketIndex(target, 4, 0)
	assert.Equal(t, 0, i)

	// should panic because forward and backward indices are out of bounds
	assert.Panics(t, func() {
		rt.chooseBucketIndex(target, 4, -1)
	})
}

func TestTable_densityBucketIndex(t *testing.T) {
	rt := newSimpleTable() // with equal idMass in each of 4 buckets
	cases := []struct {
		density           float64
		expectedBucketIdx int
	}{
		{0.0, 0},
		{0.1, 0},
		{0.25, 1},
		{0.26, 1},
		{0.51, 2},
		{0.76, 3},
		{0.77, 3},
	}
	for _, c := range cases {
		info := fmt.Sprintf("density: %v", c.density)
		assert.Equal(t, c.expectedBucketIdx, rt.densityBucketIndex(c.density), info)
	}
}

func TestTable_splitBucket(t *testing.T) {
	// try same split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		rtInt, _, _ := NewTestWithPeers(rng, int(DefaultMaxActivePeers))
		rt := rtInt.(*table)

		// lower bounds: [0 1]
		rt.splitBucket(0)
		assert.Equal(t, 2, len(rt.buckets))
		checkTableConsistent(t, rt, len(rt.peers))

		// lower bounds: [0 10 11]
		rt.splitBucket(1)
		assert.Equal(t, 3, len(rt.buckets))
		checkTableConsistent(t, rt, len(rt.peers))

		// lower bounds: [0 10 110 111]
		rt.splitBucket(2)
		assert.Equal(t, 4, len(rt.buckets))
		checkTableConsistent(t, rt, len(rt.peers))

		// lower bounds: [0 100 101 110 111]
		rt.splitBucket(1)
		assert.Equal(t, 5, len(rt.buckets))
		checkTableConsistent(t, rt, len(rt.peers))

		// lower bounds: [00 01 100 101 110 111]
		rt.splitBucket(0)
		assert.Equal(t, 6, len(rt.buckets))
		checkTableConsistent(t, rt, len(rt.peers))
	}

	// try pseudo-random split sequence with different selfIDs
	for s := 0; s < 16; s++ {
		rng := rand.New(rand.NewSource(int64(s)))
		rt, _, _ := NewTestWithPeers(rng, int(DefaultMaxActivePeers))

		// do pseudo-random splits
		for c := 0; c < 10; c++ {
			i := int(rng.Uint32()) % len(rt.(*table).buckets)
			rt.(*table).splitBucket(i)
			assert.Equal(t, c+2, len(rt.(*table).buckets))
			checkTableConsistent(t, rt, len(rt.(*table).peers))
		}
	}
}

func TestSplitLowerBound_Ok(t *testing.T) {
	check := func(lowerBound cid.ID, depth uint, expected cid.ID) {
		actual := splitLowerBound(lowerBound, depth)
		assert.Equal(t, expected, actual)
	}

	check(cid.FromInt64(0), 0, newIDLsh(1, 255))                   // no prefix
	check(newIDLsh(128, 248), 1, newIDLsh(192, 248))               // prefix 1
	check(newIDLsh(1, 254), 2, newIDLsh(3, 253))                   // prefix 01
	check(newIDLsh(3, 254), 2, newIDLsh(7, 253))                   // prefix 11
	check(newIDLsh(170, 248), 7, newIDLsh(171, 248))               // prefix 1010101
	check(newIDLsh(128, 248), 7, newIDLsh(129, 248))               // prefix 1000000
	check(newIDLsh(254, 248), 7, newIDLsh(255, 248))               // prefix 1111111
	check(newIDLsh(0, 248), 8, newIDLsh(128, 240))                 // prefix 00000000
	check(newIDLsh(128, 248), 8, newIDLsh(128<<8|128, 240))        // prefix 10000000
	check(newIDLsh(255, 248), 8, newIDLsh(255<<8|128, 240))        // prefix 11111111
	check(newIDLsh(0, 240), 9, newIDLsh(64, 240))                  // prefix 00000000 0
	check(newIDLsh(128, 240), 9, newIDLsh(192, 240))               // prefix 00000000 1
	check(newIDLsh(255, 248), 9, newIDLsh(255<<8|64, 240))         // prefix 11111111 0
	check(newIDLsh(255<<8|128, 240), 9, newIDLsh(255<<8|192, 240)) // prefix 11111111 1
}

func newSimpleTable() *table {
	return &table{
		buckets: []*bucket{
			{
				lowerBound: cid.FromInt64(0),
				upperBound: cid.FromInt64(64),
				idMass:     0.25,
				idCumMass:  0.25,
			},
			{
				lowerBound: cid.FromInt64(64),
				upperBound: cid.FromInt64(128),
				idMass:     0.25,
				idCumMass:  0.50,
			},
			{
				lowerBound: cid.FromInt64(128),
				upperBound: cid.FromInt64(192),
				idMass:     0.25,
				idCumMass:  0.75,
			},
			{
				lowerBound: cid.FromInt64(192),
				upperBound: cid.FromInt64(255),
				idMass:     0.25,
				idCumMass:  1.0,
			},
		},
	}
}

func checkTableConsistent(t *testing.T, rt Table, nExpectedPeers int) {
	nContainSelf, nPeers := 0, 0
	assert.True(t, sort.IsSorted(rt.(*table))) // buckets should be in sorted order
	assert.Equal(t, nExpectedPeers, len(rt.(*table).peers))
	assert.Equal(t, len(rt.(*table).peers), rt.(*table).NumPeers())

	idCumMass := 0.0
	for i := 0; i < len(rt.(*table).buckets); i++ {
		cur := rt.(*table).buckets[i]
		idCumMass += cur.idMass
		if i > 0 {
			// bucket boundaries should be adjacent
			prev := rt.(*table).buckets[i-1]
			assert.Equal(t, cur.lowerBound, prev.upperBound)
			assert.Equal(t, idCumMass, cur.idCumMass)
			assert.True(t, cur.idCumMass > prev.idCumMass)
		}
		if cur.containsSelf {
			nContainSelf++
		}
		checkBucketConsistent(t, cur, rt.SelfID())
		nPeers += cur.Len()

	}
	assert.Equal(t, 1.0, idCumMass)
	assert.Equal(t, nExpectedPeers, nPeers)
	assert.Equal(t, 1, nContainSelf)
}

func checkBucketConsistent(t *testing.T, b *bucket, selfID cid.ID) {
	assert.Equal(t, b.containsSelf, b.Contains(selfID))
	for p := range b.activePeers {
		pID := b.activePeers[p].ID()
		assert.True(t, pID.Cmp(b.lowerBound) >= 0)
		assert.True(t, pID.Cmp(b.upperBound) < 0)
	}
}

func newIDLsh(x int64, n uint) cid.ID {
	return cid.FromInt(new(big.Int).Lsh(big.NewInt(x), n))
}

func checkPoppedPeers(t *testing.T, k uint, numActivePeers int, ps []peer.Peer, info string) {
	if k < uint(numActivePeers) {
		// should get k peers
		assert.Equal(t, int(k), len(ps), info)

	} else {
		// should get numActivePeers
		assert.Equal(t, numActivePeers, len(ps), info)
	}

	seen := make(map[string]struct{})
	for _, p := range ps {
		assert.NotNil(t, p)

		// check we haven't previously seen this peer
		_, exists := seen[p.ID().String()]
		assert.False(t, exists)

		// mark this peer as seen
		seen[p.ID().String()] = struct{}{}
	}
}
