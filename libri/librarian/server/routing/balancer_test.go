package routing

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestTableSetBalancer_Next_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	origLC := api.NewLibrarianClient(nil)
	clients := &fixedPool{lc: origLC, getAddresses: make(map[string]struct{})}
	p, d := &fixedPreferer{}, &fixedDoctor{healthy: true}
	rt := NewEmpty(id.NewPseudoRandom(rng), p, d, NewDefaultParameters())
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-1",
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 8080},
		),
	)
	csb := NewClientBalancer(rt, clients)

	// check AddNext() returns inner LibrarianClient
	lc, address, err := csb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotEmpty(t, address)

	// check second sample returns error b/c no more unique clients
	tableSampleRetryWait = 10 * time.Millisecond // for testing
	lc, address, err = csb.AddNext()
	assert.Equal(t, client.ErrNoNewClients, err)
	assert.Nil(t, lc)
	assert.Empty(t, address)

	// add another peer, and we should be able to call Next() without error again
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-2",
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 8081},
		),
	)
	lc, address, err = csb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotEmpty(t, address)
}

func TestTableSetBalancer_Next_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p, d := &fixedPreferer{}, &fixedDoctor{healthy: true}
	rt := NewEmpty(id.NewPseudoRandom(rng), p, d, NewDefaultParameters())
	clients := &fixedPool{lc: api.NewLibrarianClient(nil), getAddresses: make(map[string]struct{})}
	cb := NewClientBalancer(rt, clients)

	// check empty RT throws error
	tableSampleRetryWait = 10 * time.Millisecond // just for test
	lc, address, err := cb.AddNext()
	assert.Equal(t, client.ErrNoNewClients, err)
	assert.Nil(t, lc)
	assert.Empty(t, address)

	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer",
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 8080},
		),
	)
	lc, address, err = cb.AddNext()
	assert.Nil(t, err)
	assert.NotNil(t, lc)
	assert.NotNil(t, address)
}

func TestTableSetBalancer_Remove(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	lc1 := api.NewLibrarianClient(nil)
	clients := &fixedPool{lc: lc1, getAddresses: make(map[string]struct{})}
	p, d := &fixedPreferer{}, &fixedDoctor{healthy: true}
	rt := NewEmpty(id.NewPseudoRandom(rng), p, d, NewDefaultParameters())
	rt.Push(
		peer.New(
			id.NewPseudoRandom(rng),
			"test-peer-1",
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 8080},
		),
	)
	csb := NewClientBalancer(rt, clients)

	lc2, address, err := csb.AddNext()
	assert.Nil(t, err)
	assert.Equal(t, lc1, lc2)
	assert.NotEmpty(t, address)
	assert.Equal(t, 1, len(csb.(*tableSetBalancer).set))

	// check removing peer from set works as expected
	err = csb.Remove(address)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(csb.(*tableSetBalancer).set))

	// check removing peer not in set errors
	err = csb.Remove("random address")
	assert.Equal(t, client.ErrClientMissingFromSet, err)
}

type fixedPool struct {
	lc           api.LibrarianClient
	getErr       error
	getAddresses map[string]struct{}
	closed       bool
}

func (fp *fixedPool) Get(address string) (api.LibrarianClient, error) {
	fp.getAddresses[address] = struct{}{}
	return fp.lc, fp.getErr
}

func (fp *fixedPool) CloseAll() error {
	fp.closed = true
	return nil
}

func (fp *fixedPool) Len() int {
	return 1
}

type fixedPreferer struct {
	prefer bool
}

func (f *fixedPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	return f.prefer
}

type fixedDoctor struct {
	healthy bool
}

func (f *fixedDoctor) Healthy(peerID id.ID) bool {
	return f.healthy
}
