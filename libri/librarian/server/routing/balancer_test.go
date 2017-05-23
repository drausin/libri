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
	origLC := api.NewLibrarianClient(nil)
	rt := NewEmpty(id.NewPseudoRandom(rng), NewDefaultParameters())
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-1",
			&fixedConnector{connectLC: origLC},
		),
	)
	csb := NewClientBalancer(rt)

	// check Next() returns inner LibrarianClient
	lc, peerID, err := csb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotNil(t, peerID)

	// check second sample returns error b/c no more unique clients
	tableSampleRetryWait = 10 * time.Millisecond // for testing
	lc, peerID, err = csb.AddNext()
	assert.Equal(t, ErrNoNewClients, err)
	assert.Nil(t, lc)
	assert.Nil(t, peerID)

	// add another peer, and we should be able to call Next() without error again
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-2",
			&fixedConnector{connectLC: origLC},
		),
	)
	lc, peerID, err = csb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotNil(t, peerID)
}

func TestTableUniqueBalancer_Next_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt := NewEmpty(id.NewPseudoRandom(rng), NewDefaultParameters())
	cb := NewClientBalancer(rt)

	// check empty RT throws error
	tableSampleRetryWait = 10 * time.Millisecond // just for test
	lc, peerID, err := cb.AddNext()
	assert.Equal(t, ErrNoNewClients, err)
	assert.Nil(t, lc)
	assert.Nil(t, peerID)

	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer",
			&fixedConnector{
				connectLC: api.NewLibrarianClient(nil),
			},
		),
	)
	lc, peerID, err = cb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotNil(t, peerID)
}

func TestRoutingTableBalancer_Remove(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	lc1 := api.NewLibrarianClient(nil)
	rt := NewEmpty(id.NewPseudoRandom(rng), NewDefaultParameters())
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-1",
			&fixedConnector{connectLC: lc1},
		),
	)
	csb := NewClientBalancer(rt)

	lc2, peerID, err := csb.AddNext()
	assert.Nil(t, err)
	assert.Equal(t, lc1, lc2)
	assert.NotNil(t, peerID)
	assert.Equal(t, 1, len(csb.(*tableSetBalancer).set))

	err = csb.Remove(peerID)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(csb.(*tableSetBalancer).set))
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
