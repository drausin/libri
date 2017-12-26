package peer

import (
	"fmt"
	"net"

	"time"

	"github.com/drausin/libri/libri/common/id"
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
	ID() id.ID

	// Address returns the public address of the peer.
	Address() *net.TCPAddr

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
	id id.ID

	address *net.TCPAddr

	// self-reported name
	name string

	// tracks query outcomes from the peer
	recorder Recorder
}

// New creates a new Peer instance with empty response stats.
func New(id id.ID, name string, address *net.TCPAddr) Peer {
	return &peer{
		id:       id,
		address:  address,
		name:     name,
		recorder: newQueryRecorder(),
	}
}

// NewStub creates a new peer without a name or connector.
func NewStub(id id.ID, name string) Peer {
	return New(id, name, nil)
}

func (p *peer) WithQueryRecorder(rec Recorder) *peer {
	p.recorder = rec
	return p
}

func (p *peer) ID() id.ID {
	return p.id
}

func (p *peer) Address() *net.TCPAddr {
	return p.address
}

func (p *peer) Before(q Peer) bool {
	pr, qr := *p.Recorder().(*queryRecorder), *q.Recorder().(*queryRecorder)
	pLatestMin := pr.responses.latest.Round(time.Minute)
	qLatestMin := qr.responses.latest.Round(time.Minute)

	// don't care about differences in latest response time within a minute
	if pLatestMin == qLatestMin {
		// p comes before q if we've made fewer queries to it, so we can attempt to balance queries
		// across peers
		return pr.responses.nQueries < qr.responses.nQueries
	}
	return pLatestMin.Before(qLatestMin)
}

func (p *peer) Merge(other Peer) error {
	if p.id.Cmp(other.ID()) != 0 {
		return fmt.Errorf("attempting to merge two different peers with IDs %v and %v",
			p.id, other.ID())
	}
	if other.(*peer).name != "" {
		p.name = other.(*peer).name
	}
	if p.Address().String() != other.Address().String() {
		p.address = other.Address()
	}
	p.recorder.Merge(other.Recorder())
	return nil
}

func (p *peer) Recorder() Recorder {
	return p.recorder
}

func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.Address()),
		QueryOutcomes: p.recorder.ToStored(),
	}
}

func (p *peer) ToAPI() *api.PeerAddress {
	return &api.PeerAddress{
		PeerId:   p.id.Bytes(),
		PeerName: p.name,
		Ip:       p.Address().IP.String(),
		Port:     uint32(p.Address().Port),
	}
}

// ToAPIs converts a list of peers into a list of api.PeerAddress objects.
func ToAPIs(peers []Peer) []*api.PeerAddress {
	addresses := make([]*api.PeerAddress, len(peers))
	for i, p := range peers {
		addresses[i] = p.ToAPI()
	}
	return addresses
}

// Fromer creates new Peer instances from api.PeerAddresses.
type Fromer interface {
	// FromAPI creates a new Peer instance.
	FromAPI(address *api.PeerAddress) Peer
}

type fromer struct{}

// NewFromer returns a new Fromer instance.
func NewFromer() Fromer {
	return &fromer{}
}

func (f *fromer) FromAPI(apiAddress *api.PeerAddress) Peer {
	return New(
		id.FromBytes(apiAddress.PeerId),
		apiAddress.PeerName,
		ToAddress(apiAddress),
	)
}

// ToAddress creates a net.TCPAddr from an api.PeerAddress.
func ToAddress(addr *api.PeerAddress) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   net.ParseIP(addr.Ip),
		Port: int(addr.Port),
	}
}

// FromAddress creates an api.PeerAddress from a net.TCPAddr.
func FromAddress(id id.ID, name string, addr *net.TCPAddr) *api.PeerAddress {
	return &api.PeerAddress{
		PeerId:   id.Bytes(),
		PeerName: name,
		Ip:       addr.IP.String(),
		Port:     uint32(addr.Port),
	}
}
