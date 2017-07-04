package routing

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
)

var benchmarkCases = []struct {
	name     string
	numPeers int
}{
	{"small", 16},
	{"medium", 64},
	{"large", 256},
}

func BenchmarkTable_Push(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkPush(b, c.numPeers) })
	}
}

func BenchmarkTable_PushPop(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkPushPop(b, c.numPeers) })
	}
}

func BenchmarkTable_Peak(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkPeak(b, c.numPeers) })
	}
}

func BenchmarkTable_Sample(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkSample(b, c.numPeers) })
	}
}

func benchmarkPush(b *testing.B, numPeers int) {
	rng := rand.New(rand.NewSource(int64(0)))
	for n := 0; n < b.N; n++ {
		rt, _, _ := NewTestWithPeers(rng, 0) // empty
		repeatedPeers := make([]peer.Peer, 0)
		for _, p := range peer.NewTestPeers(rng, numPeers) {
			status := rt.Push(p)
			if status == Added {
				// 25% of the time, we add this peer to list we'll add again later
				if rng.Float32() < 0.25 {
					repeatedPeers = append(repeatedPeers, p)
				}
			}
		}

		// add a few other peers a second time
		for _, p := range repeatedPeers {
			rt.Push(p)
		}
	}
}

func benchmarkPushPop(b *testing.B, numPeers int) {
	rng := rand.New(rand.NewSource(int64(0)))
	for n := 0; n < b.N; n++ {
		rt, _, _ := NewTestWithPeers(rng, numPeers)

		// pop half the peers every time
		for rt.NumPeers() > 0 {
			numToPop := uint(rt.NumPeers()/2 + 1)
			target := id.NewPseudoRandom(rng)
			rt.Pop(target, numToPop)
		}
	}
}

func benchmarkPeak(b *testing.B, numPeers int) {
	rng := rand.New(rand.NewSource(int64(0)))
	for n := 0; n < b.N; n++ {
		rt, _, _ := NewTestWithPeers(rng, numPeers)
		for c := 0; c < 100; c++ {
			target := id.NewPseudoRandom(rng)
			rt.Peak(target, search.DefaultNClosestResponses)
		}
	}
}

func benchmarkSample(b *testing.B, numPeers int) {
	rng := rand.New(rand.NewSource(int64(0)))
	for n := 0; n < b.N; n++ {
		rt, _, _ := NewTestWithPeers(rng, numPeers)
		for c := 0; c < 100; c++ {
			rt.Sample(introduce.DefaultNumPeersPerRequest, rng)
		}
	}
}
