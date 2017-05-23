package api

import (
	"errors"
	"math/rand"
	"net"
	"sync"
	"github.com/drausin/libri/libri/common/id"
)

// ErrEmptyLibrarianAddresses indicates that the librarian addresses is empty.
var ErrEmptyLibrarianAddresses = errors.New("empty librarian addresses")

// ClientBalancer load balances between a collection of LibrarianClients.
type ClientBalancer interface {
	// Next selects the next LibrarianClient.
	Next() (LibrarianClient, error)

	// CloseAll closes all LibrarianClient connections.
	CloseAll() error
}

// ClientSetBalancer load balances between librarian clients, ensuring that a new librarian is
// always returned.
type ClientSetBalancer interface {
	// AddNext selects the next LibrarianClient, adds it to the set, and returns it along with
	// its peer ID.
	AddNext() (LibrarianClient, id.ID, error)

	// Remove removes the librarian from the set.
	Remove(peerID id.ID) error
}

type uniformRandBalancer struct {
	rng   *rand.Rand
	mu    sync.Mutex
	conns []Connector
}

// NewUniformRandomClientBalancer creates a new ClientBalancer that selects the next client
// uniformly at random.
func NewUniformRandomClientBalancer(libAddrs []*net.TCPAddr) (ClientBalancer, error) {
	conns := make([]Connector, len(libAddrs))
	if libAddrs == nil || len(libAddrs) == 0 {
		return nil, ErrEmptyLibrarianAddresses
	}
	for i, la := range libAddrs {
		conns[i] = NewConnector(la)
	}
	return &uniformRandBalancer{
		rng:   rand.New(rand.NewSource(int64(len(conns)))),
		conns: conns,
	}, nil
}

// Next selects the next librarian client uniformly at random.
func (b *uniformRandBalancer) Next() (LibrarianClient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.rng.Int31n(int32(len(b.conns)))
	return b.conns[i].Connect()
}

func (b *uniformRandBalancer) CloseAll() error {
	for _, conn := range b.conns {
		err := conn.Disconnect()
		if err != nil {
			return err
		}
	}
	return nil
}
