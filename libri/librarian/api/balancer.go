package api

import (
	"math/rand"
	"sync"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"net"
)

// ClientBalancer load balances between a collection of LibrarianClients.
type ClientBalancer interface {
	// Next selects the next LibrarianClient.
	Next() (LibrarianClient, error)

	CloseAll() error
}

type uniformRandBalancer struct {
	rng *rand.Rand
	mu sync.Mutex
	conns []peer.Connector  // TODO (drausin) move this to api package
}

// NewUniformRandomClientBalancer creates a new ClientBalancer that selects the next client
// uniformly at random.
func NewUniformRandomClientBalancer(libAddrs []*net.TCPAddr) ClientBalancer {
	conns := make([]peer.Connector, len(libAddrs))
	for i, la := range libAddrs {
		conns[i] = peer.NewConnector(la)
	}
	return &uniformRandBalancer{
		rng: rand.New(rand.NewSource(int64(len(conns)))),
		conns: conns,
	}
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
}