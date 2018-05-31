package client

import (
	"errors"
	"math/rand"
	"net"
	"sync"

	"github.com/drausin/libri/libri/librarian/api"
)

var (
	// ErrEmptyLibrarianAddresses indicates that the librarian addresses is empty.
	ErrEmptyLibrarianAddresses = errors.New("empty librarian addresses")

	// ErrClientMissingFromSet indicates when a client ID is expected to be in a set but isn't.
	ErrClientMissingFromSet = errors.New("client missing from set")

	// ErrNoNewClients indicates when Next() cannot return a new LibrarianClient because no
	// new clients were available.
	ErrNoNewClients = errors.New("no new LibrarianClients are available")
)

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
	// its peer address string.
	AddNext() (api.LibrarianClient, string, error)

	// Remove removes the librarian from the set.
	Remove(address string) error
}

type uniformRandBalancer struct {
	rng     *rand.Rand
	mu      sync.Mutex
	addrs   []*net.TCPAddr
	clients Pool
}

// NewUniformBalancer creates a new Balancer that selects the next client
// uniformly at random.
func NewUniformBalancer(libAddrs []*net.TCPAddr, clients Pool, rng *rand.Rand) (Balancer, error) {
	if len(libAddrs) == 0 {
		return nil, ErrEmptyLibrarianAddresses
	}
	return &uniformRandBalancer{
		rng:     rng,
		clients: clients,
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

type setRandBalancer struct {
	rng       *rand.Rand
	mu        sync.Mutex
	available []string
	set       map[string]struct{}
	clients   Pool
}

// NewSetBalancer creates a new SetBalancer that selects the next client from the remaining clients
// in the available set.
func NewSetBalancer(libAddrs []*net.TCPAddr, clients Pool, rng *rand.Rand) (SetBalancer, error) {
	if len(libAddrs) == 0 {
		return nil, ErrEmptyLibrarianAddresses
	}
	available := make([]string, len(libAddrs))
	for i, libAddr := range libAddrs {
		available[i] = libAddr.String()
	}

	return &setRandBalancer{
		rng:       rng,
		clients:   clients,
		available: available,
		set:       make(map[string]struct{}),
	}, nil
}

func (b *setRandBalancer) AddNext() (api.LibrarianClient, string, error) {
	b.mu.Lock()
	if len(b.available) == 0 {
		return nil, "", ErrNoNewClients
	}
	i := b.rng.Intn(len(b.available))
	nextAddr := b.available[i]
	if i < len(b.available)-1 {
		b.available = append(b.available[:i], b.available[i+1:]...)
	} else {
		b.available = b.available[:i]
	}
	b.set[nextAddr] = struct{}{}
	b.mu.Unlock()
	lc, err := b.clients.Get(nextAddr)
	return lc, nextAddr, err
}

func (b *setRandBalancer) Remove(address string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.available = append(b.available, address)
	if _, in := b.set[address]; !in {
		return ErrClientMissingFromSet
	}
	delete(b.set, address)
	b.available = append(b.available, address)
	return nil
}
