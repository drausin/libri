package routing

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

var (
	// ErrNoNewClients indicates when Next() cannot return a new LibrarianClient because no
	// new clients were found in the table.
	ErrNoNewClients = errors.New("no new LibrarianClients found in routing table")

	tableSampleRetryWait = 2 * time.Second
	numRetries           = 16
	sampleBatchSize      = uint(8)
)

// NewClientBalancer returns a new api.ClientBalancer that uses the routing tables's Sample()
// method and returns a unique client on every Next() call.
func NewClientBalancer(rt Table) api.ClientBalancer {
	return &tableUniqueBalancer{
		rt:       rt,
		rng:      rand.New(rand.NewSource(0)),
		returned: make(map[string]struct{}),
		cache:    make([]peer.Peer, 0),
	}
}

type tableUniqueBalancer struct {
	rt       Table
	rng      *rand.Rand
	returned map[string]struct{}
	cache    []peer.Peer
	mu       sync.Mutex
}

func (b *tableUniqueBalancer) Next() (api.LibrarianClient, error) {
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
		if _, in := b.returned[nextPeer.ID().String()]; !in {
			// update internal state & return connection to new peer
			b.returned[nextPeer.ID().String()] = struct{}{}
			b.mu.Unlock()
			return nextPeer.Connector().Connect()
		}
		b.mu.Unlock()

		// wait for routing table to possibly fill up a bit
		time.Sleep(tableSampleRetryWait)
	}
	return nil, ErrNoNewClients
}

func (b *tableUniqueBalancer) CloseAll() error {
	// let the routing table handle closing its peers
	return nil
}
