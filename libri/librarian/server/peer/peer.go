package peer

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// Peer represents a peer in the network.
type Peer interface {

	// ID returns the peer ID.
	ID() cid.ID

	// Before returns whether p should be ordered before q in the priority queue of peers to
	// query. Currently, it just uses whether p's latest response time is before q's.
	Before(Peer) bool

	// Connector returns the Connector instance for connecting to the peer.
	Connector() Connector

	// Responses returns the Responses instance tracking peer responses.
	Responses() ResponseRecorder

	// ToStored returns a storage.Peer version of the peer.
	ToStored() *storage.Peer

	// ToAPI returns an api.PeerAddress version of the peer.
	ToAPI() *api.PeerAddress
}

type peer struct {
	// 256-bit ID
	id cid.ID

	// self-reported name
	name string

	// Connector instance for the peer
	conn Connector

	// time of latest response from the peer
	resp ResponseRecorder
}

// New creates a new Peer instance with empty response stats.
func New(id cid.ID, name string, conn Connector) Peer {
	return &peer{
		id:   id,
		name: name,
		conn: conn,
		resp: newResponseStats(),
	}
}

func (p *peer) WithResponseRecorder(resp ResponseRecorder) *peer {
	p.resp = resp
	return p
}

func (p *peer) ID() cid.ID {
	return p.id
}

func (p *peer) Before(q Peer) bool {
	pr, qr := p.resp.(*responseStats), q.(*peer).resp.(*responseStats)
	return pr.latest.Before(qr.latest)
}

func (p *peer) Connector() Connector {
	return p.conn
}

func (p *peer) Responses() ResponseRecorder {
	return p.resp
}

func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.conn.(*connector).publicAddress),
		Responses:     p.resp.ToStored(),
	}
}

func (p *peer) ToAPI() *api.PeerAddress {
	return &api.PeerAddress{
		PeerId:   p.id.Bytes(),
		PeerName: p.name,
		Ip:       p.conn.(*connector).publicAddress.IP.String(),
		Port:     uint32(p.conn.(*connector).publicAddress.Port),
	}
}

// Fromer creates new Peer instances from api.PeerAddresses.
type Fromer interface {
	// New creates a new Peer instance.
	FromAPI(address *api.PeerAddress) Peer
}

type fromer struct{}

// NewFromer returns a new Fromer instance.
func NewFromer() Fromer {
	return &fromer{}
}

func (f *fromer) FromAPI(apiAddress *api.PeerAddress) Peer {
	return New(
		cid.FromBytes(apiAddress.PeerId),
		apiAddress.PeerName,
		NewConnector(api.ToAddress(apiAddress)),
	)
}
