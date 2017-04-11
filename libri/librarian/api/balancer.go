package api

import (
	"math/rand"
	"sync"
)

// ClientBalancer load balances between a collection of LibrarianClients.
type ClientBalancer interface {
	Next() LibrarianClient
}

type uniformRandBalancer struct {
	rng *rand.Rand
	mu sync.Mutex
	lcs []LibrarianClient
}

func NewClientBalancer(lcs []LibrarianClient) ClientBalancer {
	return &uniformRandBalancer{
		rng: rand.New(rand.NewSource(int64(len(lcs)))),
		lcs: lcs,
	}
}

func (b *uniformRandBalancer) Next() LibrarianClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.rng.Int31n(int32(len(b.lcs)))
	return b.lcs[i]
}