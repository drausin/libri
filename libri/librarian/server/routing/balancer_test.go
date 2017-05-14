package routing

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestTableUniqueBalancer_Next_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	lc := api.NewLibrarianClient(nil)
	rt := NewEmpty(id.NewPseudoRandom(rng), NewDefaultParameters())
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-1",
			&fixedConnector{connectLC: lc},
		),
	)
	cb := NewClientBalancer(rt)

	// check Next() returns inner LibrarianClient
	lc, err := cb.Next()
	assert.Nil(t, err)
	assert.NotNil(t, lc)

	// check second sample returns error b/c no more unique clients
	tableSampleRetryWait = 10 * time.Millisecond // for testing
	lc1, err := cb.Next()
	assert.Equal(t, ErrNoNewClients, err)
	assert.Nil(t, lc1)

	// add another peer, and we should be able to call Next() without error again
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-2",
			&fixedConnector{connectLC: lc},
		),
	)
	lc2, err := cb.Next()
	assert.Nil(t, err)
	assert.NotNil(t, lc2)
}

func TestTableUniqueBalancer_Next_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt := NewEmpty(id.NewPseudoRandom(rng), NewDefaultParameters())
	cb := NewClientBalancer(rt)

	// check empty RT throws error
	tableSampleRetryWait = 10 * time.Millisecond // just for test
	lc, err := cb.Next()
	assert.Equal(t, ErrNoNewClients, err)
	assert.Nil(t, lc)

	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer",
			&fixedConnector{
				connectLC: api.NewLibrarianClient(nil),
			},
		),
	)
	lc, err = cb.Next()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
}
func TestRoutingTableBalancer_CloseAll(t *testing.T) {
	cb := NewClientBalancer(nil)
	err := cb.CloseAll()
	assert.Nil(t, err)
}

type fixedConnector struct {
	connectLC  api.LibrarianClient
	connectErr error
}

func (fc *fixedConnector) Connect() (api.LibrarianClient, error) {
	return fc.connectLC, fc.connectErr
}

// Disconnect closes the connection with the peer.
func (fc *fixedConnector) Disconnect() error {
	return nil
}

// Address returns the TCP address used by the Connector.
func (fc *fixedConnector) Address() *net.TCPAddr {
	return nil
}
