package peer

import (
	"net"

	"sync"

	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Connector creates and destroys connections with a peer.
type Connector interface {
	// Connect establishes the TCP connection with the peer if it doesn't already exist
	// and returns an api.LibrarianClient.
	Connect() (api.LibrarianClient, error)

	// Disconnect closes the connection with the peer.
	Disconnect() error

	// Address returns the TCP address used by the Connector.
	Address() *net.TCPAddr

	merge(other Connector) error

	connected() bool

	ready() bool
}

type connector struct {
	// RPC TCP address
	publicAddress *net.TCPAddr

	// Librarian client to peer
	client api.LibrarianClient

	// client connection to the peer
	clientConn *grpc.ClientConn

	// dials a particular address
	dialer dialer

	// for getting clientConn connectivity state
	stateGetter clientConnStateGetter

	mu *sync.Mutex
}

// NewConnector creates a Connector instance from an address.
func NewConnector(address *net.TCPAddr) Connector {
	return &connector{
		publicAddress: address,
		dialer:        insecureDialer{},
		stateGetter:   clientConnStateGetterImpl{},
		mu:            new(sync.Mutex),
	}
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (c *connector) Connect() (api.LibrarianClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		conn, err := c.dialer.Dial(c.publicAddress)
		if err != nil {
			return nil, err
		}
		c.client = api.NewLibrarianClient(conn)
		c.clientConn = conn
	}
	return c.client, nil
}

// Disconnect closes the connection with the peer.
func (c *connector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		c.client = nil
		return c.clientConn.Close()
	}
	return nil
}

func (c *connector) merge(other Connector) error {
	if c.Address().String() != other.Address().String() {
		// replace if address is different
		return c.replaceWith(other)
	}

	// prefer the more connected/ready of the two connectors
	selfConnected, otherConnected := c.connected(), other.connected()
	if !selfConnected && otherConnected {
		return c.replaceWith(other)
	}
	if selfConnected && !otherConnected {
		return nil
	}
	if !selfConnected && !otherConnected {
		// no need to do anything if neither is connected
		return nil
	}
	if selfConnected && otherConnected {
		if !c.ready() && other.ready() {
			// only replace if other's ready and we're not
			return c.replaceWith(other)
		}
		// keep existing and clean up other
		return other.Disconnect()
	}

	// should only get here in weird race conditions; disconnect other just to be safe
	return other.Disconnect()
}

func (c *connector) replaceWith(other Connector) error {
	if err := c.Disconnect(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// we never should be replacing with a Connector that's not of type *connector; if this ever
	// changes, we'll get a panic here
	c.client = other.(*connector).client
	c.clientConn = other.(*connector).clientConn
	c.publicAddress = other.(*connector).publicAddress
	return nil
}

func (c *connector) connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientConn != nil
}

func (c *connector) ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientConn != nil && c.clientConn.GetState() == connectivity.Ready
}

func (c *connector) Address() *net.TCPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.publicAddress
}

type dialer interface {
	Dial(addr *net.TCPAddr) (*grpc.ClientConn, error)
}

type insecureDialer struct{}

func (insecureDialer) Dial(addr *net.TCPAddr) (*grpc.ClientConn, error) {
	return grpc.Dial(addr.String(), grpc.WithInsecure())
}

// very thin wrapper for testing since grpc.connectivityStateManager is private
type clientConnStateGetter interface {
	get(cc *grpc.ClientConn) connectivity.State
}

type clientConnStateGetterImpl struct{}

func (sg clientConnStateGetterImpl) get(cc *grpc.ClientConn) connectivity.State {
	return cc.GetState()
}
