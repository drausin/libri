package peer

import (
	"fmt"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

const (
	// MissingName is the placeholder for a missing peer name.
	MissingName = "MISING_NAME"
)

// Peer represents a peer in the network.
type Peer interface {

	// ID returns the peer ID.
	ID() cid.ID

	// Connector returns the Connector instance for connecting to the peer.
	Connector() Connector

	// Recorder returns the Recorder instance for recording query outcomes.
	Recorder() Recorder

	// Before returns whether p should be ordered before q in the priority queue of peers to
	// query. Currently, it just uses whether p's latest response time is before q's.
	Before(other Peer) bool

	// Merge merges another peer into the existing peer. If there is any conflicting information
	// between the two, the merge returns an error.
	Merge(other Peer) error

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

	// tracks query outcomes from the peer
	recorder Recorder
}

// New creates a new Peer instance with empty response stats.
func New(id cid.ID, name string, conn Connector) Peer {
	return &peer{
		id:       id,
		name:     name,
		conn:     conn,
		recorder: newQueryRecorder(),
	}
}

// NewStub creates a new peer without a name or connector.
func NewStub(id cid.ID, name string) Peer {
	return New(id, name, nil)
}

func (p *peer) WithQueryRecorder(rec Recorder) *peer {
	p.recorder = rec
	return p
}

func (p *peer) ID() cid.ID {
	return p.id
}

func (p *peer) Before(q Peer) bool {
	pr, qr := p.recorder.(*queryRecorder), q.(*peer).recorder.(*queryRecorder)
	return pr.responses.latest.Before(qr.responses.latest)
}

func (p *peer) Merge(other Peer) error {
	if p.id.Cmp(other.ID()) != 0 {
		return fmt.Errorf("attempting to merge two different peers with IDs %v and %v",
			p.id, other.ID())
	}

	if other.(*peer).name != "" {
		p.name = other.(*peer).name
	}
	pAddr := p.conn.(*connector).publicAddress
	if other.Connector() != nil && pAddr != other.Connector().(*connector).publicAddress {
		otherAddr := other.Connector().(*connector).publicAddress
		return fmt.Errorf("unable to merge public addresses for peer %v: existing (%v)"+
			" conflicts with new (%v)", p.id, pAddr.String(), otherAddr.String())
	}
	p.recorder.Merge(other.Recorder())
	return nil
}

func (p *peer) Connector() Connector {
	return p.conn
}

func (p *peer) Recorder() Recorder {
	return p.recorder
}

func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.conn.(*connector).publicAddress),
		QueryOutcomes: p.recorder.ToStored(),
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
