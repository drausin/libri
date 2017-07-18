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

// GetterBalancer load balances between a collection of Getters.
type GetterBalancer interface {
	// Next selects the next Getter.
	Next() (Getter, error)
}

// PutterBalancer laod balances between a collection of Putters.
type PutterBalancer interface {
	// Next selects the next Putter.
	Next() (Putter, error)
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
	addrs []*net.TCPAddr
	conns []Connector
}

// NewUniformClientBalancer creates a new ClientBalancer that selects the next client
// uniformly at random.
func NewUniformClientBalancer(libAddrs []*net.TCPAddr) (ClientBalancer, error) {
	conns := make([]Connector, len(libAddrs))
	if len(libAddrs) == 0 {
		return nil, ErrEmptyLibrarianAddresses
	}
	return &uniformRandBalancer{
		rng:   rand.New(rand.NewSource(int64(len(conns)))),
		conns: conns,
		addrs: libAddrs,
	}, nil
}

// Next selects the next librarian client uniformly at random.
func (b *uniformRandBalancer) Next() (LibrarianClient, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.rng.Int31n(int32(len(b.conns)))
	if b.conns[i] == nil {
		// only init when needed
		b.conns[i] = NewConnector(b.addrs[i])
	}
	return b.conns[i].Connect()
}

func (b *uniformRandBalancer) CloseAll() error {
	for _, conn := range b.conns {
		if conn != nil {
			err := conn.Disconnect()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type uniformGetterBalancer struct {
	inner ClientBalancer
}

// NewUniformGetterBalancer creates a GetterBalancer using an internal ClientBalancer.
func NewUniformGetterBalancer(inner ClientBalancer) GetterBalancer {
	return &uniformGetterBalancer{inner}
}

func (b *uniformGetterBalancer) Next() (Getter, error) {
	next, err := b.inner.Next()
	if err != nil {
		return nil, err
	}
	return next.(Getter), nil
}

type uniformPutterBalancer struct {
	inner ClientBalancer
}

// NewUniformPutterBalancer creates a PutterBalancer using an internal ClientBalancer.
func NewUniformPutterBalancer(inner ClientBalancer) PutterBalancer {
	return &uniformPutterBalancer{inner}
}

func (b *uniformPutterBalancer) Next() (Putter, error) {
	next, err := b.inner.Next()
	if err != nil {
		return nil, err
	}
	return next.(Putter), nil
}
