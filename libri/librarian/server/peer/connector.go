package peer

import (
	"net"

	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
)

// Connector creates and destroys connections with a peer.
type Connector interface {

	// PublicAddress returns the public address to connect to.
	PublicAddress() *net.TCPAddr

	// Connect establishes the TCP connection with the peer if it doesn't already exist
	// and returns an api.LibrarianClient.
	Connect() (api.LibrarianClient, error)

	// Disconnect closes the connection with the peer.
	Disconnect() error
}

type connector struct {

	// RPC TCP address
	publicAddress *net.TCPAddr

	// Librarian client to peer
	client api.LibrarianClient

	// client connection to the peer
	clientConn *grpc.ClientConn
}

// NewConnector creates a Connector instance from an address.
func NewConnector(address *net.TCPAddr) Connector {
	return &connector{publicAddress: address}
}

func (c *connector) PublicAddress() *net.TCPAddr {
	return c.publicAddress
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (c *connector) Connect() (api.LibrarianClient, error) {
	if c.client == nil {
		// TODO (drausin) add SSL
		conn, err := grpc.Dial(c.publicAddress.String(), grpc.WithInsecure())
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
	if c.client != nil {
		c.client = nil
		return c.clientConn.Close()
	}
	return nil
}
