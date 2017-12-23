package client

import (
	"io"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/hashicorp/golang-lru"
	"google.golang.org/grpc"
)

const (
	defaultMaxConns = 128
)

// Pool maintains a pool of librarian clients.
type Pool interface {
	// Get the connection to the given address.
	Get(address string) (api.LibrarianClient, error)

	// CloseAll closes all active connections. Not further Get() calls may be made after this call.
	CloseAll() error
}

type lruPool struct {
	conns       *lru.Cache
	dialer      dialer
	closer      closer
	evictionErr chan error
}

// NewLRUPool creates a new LRU Pool with the given number of max connections.
func NewLRUPool(maxConns int) (Pool, error) {
	return newLRUPool(maxConns, insecureDialer{}, closerImpl{})
}

// NewDefaultLRUPool creates a new LRU pool with the default number of max connections.
func NewDefaultLRUPool() (Pool, error) {
	return NewLRUPool(defaultMaxConns)
}

func newLRUPool(maxConns int, dialer dialer, closer closer) (Pool, error) {
	evictionErrs := make(chan error, 1)
	onEvicted := func(key interface{}, value interface{}) {
		// close connection when it is evicted
		evictionErrs <- closer.close(value.(*grpc.ClientConn))
	}
	conns, err := lru.NewWithEvict(maxConns, onEvicted)
	if err != nil {
		return nil, err
	}
	return &lruPool{
		conns:       conns,
		dialer:      dialer,
		closer:      closer,
		evictionErr: evictionErrs,
	}, nil
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
	if eviction := p.conns.Add(address, conn); eviction {
		if err := <-p.evictionErr; err != nil {
			return nil, err
		}
	}
	return api.NewLibrarianClient(conn), nil
}

func (p *lruPool) CloseAll() error {
	go func() {
		p.conns.Purge()
		close(p.evictionErr)
	}()

	for err := range p.evictionErr {
		if err != nil {
			return err
		}
	}
	return nil
}

// dialer is a very thin wrapper around grpc.Dial to facilitate mocking during testing
type dialer interface {
	dial(address string) (*grpc.ClientConn, error)
}

type insecureDialer struct{}

func (insecureDialer) dial(address string) (*grpc.ClientConn, error) {
	return grpc.Dial(address, grpc.WithInsecure())
}

// closer is a very thin wrapper around (*grpc.ClientConn).Close() to facilitate mocking during
// testing
type closer interface {
	close(conn io.Closer) error
}

type closerImpl struct{}

func (closerImpl) close(conn io.Closer) error {
	return conn.Close()
}
