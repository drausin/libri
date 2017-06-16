package routing

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

var (
	// ErrNoNewClients indicates when Next() cannot return a new LibrarianClient because no
	// new clients were found in the table.
	ErrNoNewClients = errors.New("no new LibrarianClients found in routing table")

	// ErrClientMissingFromSet indicates when a client ID is expected to be in a set but isn't.
	ErrClientMissingFromSet = errors.New("client missing from set")

	tableSampleRetryWait = 15 * time.Second
	numRetries           = 32
	sampleBatchSize      = uint(8)
)

// NewClientBalancer returns a new api.ClientBalancer that uses the routing tables's Sample()
// method and returns a unique client on every Next() call.
func NewClientBalancer(rt Table) api.ClientSetBalancer {
	return &tableSetBalancer{
		rt:    rt,
		rng:   rand.New(rand.NewSource(0)),
		set:   make(map[string]struct{}),
		cache: make([]peer.Peer, 0),
	}
}

type tableSetBalancer struct {
	rt    Table
	rng   *rand.Rand
	set   map[string]struct{}
	cache []peer.Peer
	mu    sync.Mutex
}

func (b *tableSetBalancer) AddNext() (api.LibrarianClient, id.ID, error) {
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
		if _, in := b.set[nextPeer.ID().String()]; !in {
			// update current state & return connection to new peer
			b.set[nextPeer.ID().String()] = struct{}{}
			b.mu.Unlock()
			lc, err := nextPeer.Connector().Connect()
			return lc, nextPeer.ID(), err
		}
		b.mu.Unlock()

		// wait for routing table to possibly fill up a bit
		time.Sleep(tableSampleRetryWait)
	}
	return nil, nil, ErrNoNewClients
}

func (b *tableSetBalancer) Remove(peerID id.ID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, in := b.set[peerID.String()]; !in {
		return ErrClientMissingFromSet
	}
	delete(b.set, peerID.String())
	return nil
}
