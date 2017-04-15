package api

import (
	"net"

	"google.golang.org/grpc"
)

// Connector creates and destroys connections with a peer.
type Connector interface {
	// Connect establishes the TCP connection with the peer if it doesn't already exist
	// and returns an api.LibrarianClient.
	Connect() (LibrarianClient, error)

	// Disconnect closes the connection with the peer.
	Disconnect() error

	// Address returns the TCP address used by the Connector.
	Address() *net.TCPAddr
}

type connector struct {

	// RPC TCP address
	publicAddress *net.TCPAddr

	// Librarian client to peer
	client        LibrarianClient

	// client connection to the peer
	clientConn    *grpc.ClientConn

	// dials a particular address
	dialer        dialer
}

// NewConnector creates a Connector instance from an address.
func NewConnector(address *net.TCPAddr) Connector {
	return &connector{
		publicAddress: address,
		dialer: insecureDialer{},
	}
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (c *connector) Connect() (LibrarianClient, error) {
	if c.client == nil {
		conn, err := c.dialer.Dial(c.publicAddress)
		if err != nil {
			return nil, err
		}
		c.client = NewLibrarianClient(conn)
		c.clientConn = conn
	}
	return c.client, nil
}

// Disconnect closes the connection with the peer.
func (c *connector) Disconnect() error {
	if c.client != nil {
		c.client = nil
		return c.clientConn.Close()
	}
	return nil
}

func (c *connector) Address() *net.TCPAddr {
	return c.publicAddress
}


type dialer interface {
	Dial(addr *net.TCPAddr) (*grpc.ClientConn, error)
}

type insecureDialer struct {}

func (insecureDialer) Dial(addr *net.TCPAddr) (*grpc.ClientConn, error) {
	return grpc.Dial(addr.String(), grpc.WithInsecure())
}
