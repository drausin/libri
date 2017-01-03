package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
)

var (
	// IDUpperBound is the upper bound of the ID space, i.e., all 256 bits on.
	IDUpperBound = NewID(bytes.Repeat([]byte{255}, IDLength))

	// IDLowerBound is the lower bound of the ID space, i.e., all 256 bits off.
	IDLowerBound = NewID(make([]byte, IDLength))
)

// Peer represents a peer in the network.
type Peer struct {
	// 256-bit ID
	ID *big.Int

	// string encoding of ID
	IDStr string

	// self-reported name
	Name string

	// RPC TCP address
	PublicAddress *net.TCPAddr

	// Librarian client to peer
	Client *api.LibrarianClient

	// client connection to the peer
	conn *grpc.ClientConn

	// time of latest response from the peer
	LatestResponse time.Time
}

// NewPeer creates a new Peer instance.
func NewPeer(id *big.Int, name string, publicAddress *net.TCPAddr,
	latestResponse time.Time) (*Peer, error) {
	if latestResponse.Location() != time.UTC {
		return nil, fmt.Errorf("latestResponse should have a UTC location, instead found "+
			"%v", latestResponse.Location())
	}

	return &Peer{
		ID:             id,
		IDStr:          IDString(id),
		Name:           name,
		PublicAddress:  publicAddress,
		LatestResponse: latestResponse,
	}, nil
}

// NewPeerFromStorage creates a new Peer instance from a StoredPeer instance.
func NewPeerFromStorage(stored *StoredPeer) (*Peer, error) {
	return NewPeer(
		big.NewInt(0).SetBytes(stored.Id),
		stored.Name,
		&net.TCPAddr{
			IP:   net.ParseIP(stored.AddressIp),
			Port: int(stored.AddressPort),
		},
		time.Unix(stored.LatestResponse, int64(0)).UTC(),
	)
}

// NewStoredPeer creates a new StoredPeer instance from the Peer instance.
func (peer *Peer) NewStoredPeer() *StoredPeer {
	return &StoredPeer{
		Id:             peer.ID.Bytes(),
		Name:           peer.Name,
		AddressIp:      peer.PublicAddress.IP.String(),
		AddressPort:    uint32(peer.PublicAddress.Port),
		LatestResponse: peer.LatestResponse.Unix(),
	}
}

// NewAPIPeer creates a new api.Peer (from protobuf) instance from the Peer instance.
func (peer *Peer) NewAPIPeer() *api.Peer {
	return &api.Peer{
		PeerId:      peer.ID.Bytes(),
		AddressIp:   peer.PublicAddress.IP.String(),
		AddressPort: uint32(peer.PublicAddress.Port),
	}
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (peer *Peer) Connect() error {
	// TODO (drausin) add SSL
	conn, err := grpc.Dial(peer.PublicAddress.String(), grpc.WithInsecure())
	if err != nil {
		return err
	}
	client := api.NewLibrarianClient(conn)
	peer.conn = conn
	peer.Client = &client
	return nil
}

// Disconnect closes the connection with the peer.
func (peer *Peer) Disconnect() error {
	if peer.Client != nil {
		peer.Client = nil
		return peer.conn.Close()
	}
	return nil
}

// IDString gives the string encoding of the ID.
func IDString(id *big.Int) string {
	return base64.URLEncoding.EncodeToString(id.Bytes())
}
