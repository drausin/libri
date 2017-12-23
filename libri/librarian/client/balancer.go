package client

import (
	"errors"
	"math/rand"
	"net"
	"sync"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// ErrEmptyLibrarianAddresses indicates that the librarian addresses is empty.
var ErrEmptyLibrarianAddresses = errors.New("empty librarian addresses")

// Balancer load balances between a collection of LibrarianClients.
type Balancer interface {
	// Next selects the next LibrarianClient.
	Next() (api.LibrarianClient, error)

	// CloseAll closes all LibrarianClient connections.
	CloseAll() error
}

// GetterBalancer load balances between a collection of Getters.
type GetterBalancer interface {
	// Next selects the next Getter.
	Next() (api.Getter, error)
}

// PutterBalancer laod balances between a collection of Putters.
type PutterBalancer interface {
	// Next selects the next Putter.
	Next() (api.Putter, error)
}

// SetBalancer load balances between librarian clients, ensuring that a new librarian is
// always returned.
type SetBalancer interface {
	// AddNext selects the next LibrarianClient, adds it to the set, and returns it along with
	// its peer ID.
	AddNext() (api.LibrarianClient, id.ID, error)

	// Remove removes the librarian from the set.
	Remove(peerID id.ID) error
}

type uniformRandBalancer struct {
	rng     *rand.Rand
	mu      sync.Mutex
	addrs   []*net.TCPAddr
	clients Pool
}

// NewUniformBalancer creates a new Balancer that selects the next client
// uniformly at random.
func NewUniformBalancer(libAddrs []*net.TCPAddr, rng *rand.Rand) (Balancer, error) {
	conns, err := NewDefaultLRUPool()
	if err != nil {
		return nil, err
	}
	if len(libAddrs) == 0 {
		return nil, ErrEmptyLibrarianAddresses
	}
	return &uniformRandBalancer{
		rng:     rng,
		clients: conns,
		addrs:   libAddrs,
	}, nil
}

// Next selects the next librarian client uniformly at random.
func (b *uniformRandBalancer) Next() (api.LibrarianClient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.rng.Int31n(int32(len(b.addrs)))
	return b.clients.Get(b.addrs[i].String())
}

func (b *uniformRandBalancer) CloseAll() error {
	return b.clients.CloseAll()
}

type uniformGetterBalancer struct {
	inner Balancer
}

// NewUniformGetterBalancer creates a GetterBalancer using an internal Balancer.
func NewUniformGetterBalancer(inner Balancer) GetterBalancer {
	return &uniformGetterBalancer{inner}
}

func (b *uniformGetterBalancer) Next() (api.Getter, error) {
	next, err := b.inner.Next()
	if err != nil {
		return nil, err
	}
	return next.(api.Getter), nil
}

type uniformPutterBalancer struct {
	inner Balancer
}

// NewUniformPutterBalancer creates a PutterBalancer using an internal Balancer.
func NewUniformPutterBalancer(inner Balancer) PutterBalancer {
	return &uniformPutterBalancer{inner}
}

func (b *uniformPutterBalancer) Next() (api.Putter, error) {
	next, err := b.inner.Next()
	if err != nil {
		return nil, err
	}
	return next.(api.Putter), nil
}
