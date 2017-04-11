package api

import (
	"math/rand"
	"sync"
)

// ClientBalancer load balances between a collection of LibrarianClients.
type ClientBalancer interface {
	// Next() selects the next LibrarianClient.
	Next() LibrarianClient
}

type uniformRandBalancer struct {
	rng *rand.Rand
	mu sync.Mutex
	lcs []LibrarianClient
}

// NewUniformRandomClientBalancer creates a new ClientBalancer that selects the next client
// uniformly at random.
func NewUniformRandomClientBalancer(lcs []LibrarianClient) ClientBalancer {
	return &uniformRandBalancer{
		rng: rand.New(rand.NewSource(int64(len(lcs)))),
		lcs: lcs,
	}
}

// Next selects the next librarian client uniformly at random.
func (b *uniformRandBalancer) Next() LibrarianClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.rng.Int31n(int32(len(b.lcs)))
	return b.lcs[i]
}