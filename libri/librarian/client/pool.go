package client

import (
	"google.golang.org/grpc"
	"github.com/hashicorp/golang-lru"
	"github.com/drausin/libri/libri/librarian/api"
)

const (
	defaultMaxConns = 128
)

// Pool maintains a pool of librarian clients.
type Pool interface {
	// Get the connection to the given address.
	Get(address string) (api.LibrarianClient, error)

	// CloseAll closes all active connections.
	CloseAll() error
}

type lruPool struct {
	conns  *lru.Cache
	dialer dialer
	errs   chan error
}

func NewLRUPool(maxConns int) (Pool, error) {
	p := &lruPool{
		dialer: insecureDialer{},
		errs:   make(chan error, 1),
	}
	onEvicted := func(key interface{}, value interface{}) {
		// close connection when it is evicted
		if err := value.(*grpc.ClientConn).Close(); err != nil {
			p.errs <- err
		}
	}
	conns, err := lru.NewWithEvict(maxConns, onEvicted)
	if err != nil {
		return nil, err
	}
	p.conns = conns
	return p, nil
}

func NewDefaultLRUPool() (Pool, error) {
	return NewLRUPool(defaultMaxConns)
}

func (p *lruPool) Get(address string) (api.LibrarianClient, error) {
	if value, in := p.conns.Get(address); in {
		// return existing connection if we have it
		return api.NewLibrarianClient(value.(*grpc.ClientConn)), nil
	}
	// create a new connection
	conn, err := p.dialer.dial(address)
	if err != nil {
		return nil, err
	}
	p.conns.Add(address, conn)
	select {
	case err := <-p.errs:
		// receive error from eviction
		return nil, err
	default:
		return api.NewLibrarianClient(conn), nil
	}
}

func (p *lruPool) CloseAll() error {
	for _, address := range p.conns.Keys() {
		conn, _ := p.conns.Get(address)
		if err := conn.(*grpc.ClientConn).Close(); err != nil {
			return err
		}
		p.conns.Remove(address)
	}
	return nil
}

func (p *lruPool) close(conn *grpc.ClientConn) {
}

type dialer interface {
	dial(address string) (*grpc.ClientConn, error)
}

type insecureDialer struct{}

func (insecureDialer) dial(address string) (*grpc.ClientConn, error) {
	return grpc.Dial(address, grpc.WithInsecure())
}
