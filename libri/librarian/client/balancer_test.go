package client

import (
	"math/rand"
	"net"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewUniformBalancer_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	b, err := NewUniformBalancer([]*net.TCPAddr{}, rng)
	assert.Equal(t, ErrEmptyLibrarianAddresses, err)
	assert.Nil(t, b)
}

func TestUniformRandBalancer_Next(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	addrs := []*net.TCPAddr{
		{IP: net.ParseIP("1.2.3.4"), Port: 8080},
		{IP: net.ParseIP("1.2.3.4"), Port: 8081},
		{IP: net.ParseIP("1.2.3.4"), Port: 8082},
	}
	b, err := NewUniformBalancer(addrs, rng)
	assert.Nil(t, err)
	assert.NotNil(t, b)
	lc := api.NewLibrarianClient(nil)
	clients := &fixedPool{lc: lc, getAddresses: make(map[string]struct{})}
	b.(*uniformRandBalancer).clients = clients

	for c := 0; c < 16; c++ { // should be enough trials to hit each addr at least once
		lc, err := b.Next()
		assert.Nil(t, err)
		assert.NotNil(t, lc)
	}

	assert.Len(t, clients.getAddresses, 3)
}

func TestUniformRandBalancer_CloseAll(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	addrs := []*net.TCPAddr{
		{IP: net.ParseIP("1.2.3.4"), Port: 8080},
		{IP: net.ParseIP("1.2.3.4"), Port: 8081},
		{IP: net.ParseIP("1.2.3.4"), Port: 8082},
	}
	b, err := NewUniformBalancer(addrs, rng)
	assert.Nil(t, err)
	assert.NotNil(t, b)
	clients := &fixedPool{}
	b.(*uniformRandBalancer).clients = clients

	err = b.CloseAll()
	assert.Nil(t, err)
	assert.True(t, clients.closed)
}

func TestUniformGetterBalancer_Next(t *testing.T) {
	okBalancer := &fixedBalancer{client: api.NewLibrarianClient(nil)}
	b1 := NewUniformGetterBalancer(okBalancer)
	g, err := b1.Next()
	assert.Nil(t, err)
	assert.Equal(t, okBalancer.client, g)

	badBalancer := &fixedBalancer{err: errors.New("some Next() error")}
	b2 := NewUniformGetterBalancer(badBalancer)
	g, err = b2.Next()
	assert.NotNil(t, err)
	assert.Nil(t, g)
}

func TestUniformPutterBalancer_Next(t *testing.T) {
	okBalancer := &fixedBalancer{client: api.NewLibrarianClient(nil)}
	b1 := NewUniformPutterBalancer(okBalancer)
	p, err := b1.Next()
	assert.Nil(t, err)
	assert.Equal(t, okBalancer.client, p)

	badBalancer := &fixedBalancer{err: errors.New("some Next() error")}
	b2 := NewUniformPutterBalancer(badBalancer)
	p, err = b2.Next()
	assert.NotNil(t, err)
	assert.Nil(t, p)
}

type fixedPool struct {
	lc api.LibrarianClient
	getErr error
	getAddresses map[string]struct{}
	closed bool
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

type fixedBalancer struct {
	client api.LibrarianClient
	err    error
}

func (f *fixedBalancer) Next() (api.LibrarianClient, error) {
	return f.client, f.err
}

func (f *fixedBalancer) CloseAll() error {
	return nil
}
