package peer

import (
	cid "github.com/drausin/libri/libri/common/id"
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
}

type peer struct {
	// 256-bit ID
	id cid.ID

	// self-reported name
	name string

	// Connector instance for the peer
	connector Connector

	// time of latest response from the peer
	responses ResponseRecorder
}

// New creates a new Peer instance with empty response stats.
func New(id cid.ID, name string, connector Connector) Peer {
	return NewWithResponseStats(id, name, connector, newResponseStats())
}

// NewWithResponseStats creates a new Peer instance with the given response stats.
func NewWithResponseStats(id cid.ID, name string, connector Connector,
	responses ResponseRecorder) Peer {
	return &peer{
		id:        id,
		name:      name,
		connector: connector,
		responses: responses,
	}
}

func (p *peer) ID() cid.ID {
	return p.id
}

func (p *peer) Before(q Peer) bool {
	pr, qr := p.responses.(*responseStats), q.(*peer).responses.(*responseStats)
	return pr.latest.Before(qr.latest)
}

func (p *peer) Connector() Connector {
	return p.connector
}

func (p *peer) Responses() ResponseRecorder {
	return p.responses
}

func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.connector.PublicAddress()),
		Responses:     p.responses.ToStored(),
	}
}
