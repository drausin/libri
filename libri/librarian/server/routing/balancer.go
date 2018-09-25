package routing

import (
	"math/rand"
	"sync"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

var (
	tableSampleRetryWait = 15 * time.Second
	numRetries           = 32
	sampleBatchSize      = uint(8)
)

// NewClientBalancer returns a new client.Balancer that uses the routing tables's Sample()
// method and returns a unique client on every Next() call.
func NewClientBalancer(rt Table, clients client.Pool) client.SetBalancer {
	return &tableSetBalancer{
		rt:      rt,
		rng:     rand.New(rand.NewSource(0)),
		set:     make(map[string]struct{}),
		cache:   make([]peer.Peer, 0),
		clients: clients,
	}
}

type tableSetBalancer struct {
	rt      Table
	rng     *rand.Rand
	set     map[string]struct{}
	cache   []peer.Peer
	clients client.Pool
	mu      sync.Mutex
}

func (b *tableSetBalancer) AddNext() (api.LibrarianClient, string, error) {
	for c := 0; c < numRetries; c++ {
		b.mu.Lock()
		if len(b.cache) == 0 {
			b.cache = b.rt.Sample(sampleBatchSize, b.rng)
			if len(b.cache) == 0 {
				b.mu.Unlock()
				// wait for routing table to possibly fill up a bit
				time.Sleep(tableSampleRetryWait)
				continue
			}
		}
		nextPeer := b.cache[0]
		b.cache = b.cache[1:]
		nextAddress := nextPeer.Address().String()
		if _, in := b.set[nextAddress]; !in {
			// update current state & return connection to new peer
			b.set[nextAddress] = struct{}{}
			b.mu.Unlock()
			lc, err := b.clients.Get(nextAddress)
			return lc, nextAddress, err
		}
		b.mu.Unlock()

		// wait for routing table to possibly fill up a bit
		time.Sleep(tableSampleRetryWait)
	}
	return nil, "", client.ErrNoNewClients
}

func (b *tableSetBalancer) Remove(address string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, in := b.set[address]; !in {
		return client.ErrClientMissingFromSet
	}
	delete(b.set, address)
	return nil
}
