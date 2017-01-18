package peer

import (
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// Peer represents a peer in the network.
type Peer interface {

	// ID returns the peer ID.
	ID() cid.ID

	// Name returns the self-reported name.
	Name() string

	// ResponseStats returns the peer's response stats.
	ResponseStats() *responseStats

	// Before returns whether p should be ordered before q in the priority queue of peers to
	// query. Currently, it just uses whether p's latest response time is before q's.
	Before(Peer) bool

	// RecordResponseSuccess records a successful response from the peer.
	RecordResponseSuccess()

	// RecordResponseError records an unsuccessful or error-laden response from the peer.
	RecordResponseError()

	// Connector returns the Connector instance for connecting to the peer.
	Connector() Connector

	// ToStored returns a storage.Peer version of the peer.
	ToStored() *storage.Peer
}

type peer struct {
	// 256-bit ID
	id cid.ID

	// self-reported name
	name string

	// time of latest response from the peer
	responses *responseStats

	// Connector instance for the peer
	conn Connector
}

// New creates a new Peer instance with empty response stats.
func New(id cid.ID, name string, conn Connector) Peer {
	return NewWithResponseStats(id, name, conn, newResponseStats())
}

// NewWithResponseStats creates a new Peer instance with the given response stats.
func NewWithResponseStats(id cid.ID, name string, conn Connector, rs *responseStats) Peer {
	return &peer{
		id:        id,
		name:      name,
		conn:      conn,
		responses: rs,
	}
}

func (p *peer) ID() cid.ID {
	return p.id
}

func (p *peer) Name() string {
	return p.name
}

func (p *peer) ResponseStats() *responseStats {
	return p.responses
}

func (p *peer) Before(q Peer) bool {
	return p.ResponseStats().latest.Before(q.ResponseStats().latest)
}

func (p *peer) RecordResponseSuccess() {
	p.recordResponse()
}

func (p *peer) RecordResponseError() {
	p.responses.nErrors++
	p.recordResponse()
}

func (p *peer) Connector() Connector {
	return p.conn
}

func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.conn.PublicAddress()),
		Responses:     p.responses.toStoredResponseStats(),
	}
}

// recordsResponse records any response from the peer.
func (p *peer) recordResponse() {
	p.responses.nQueries++
	p.responses.latest = time.Now().UTC()
	if p.responses.earliest.Unix() == 0 {
		p.responses.earliest = p.responses.latest
	}
}

// ResponseStats describes metrics associated with a peer's communication history.
type responseStats struct {
	// earliest response time from the peer
	earliest time.Time

	// latest response time form the peer
	latest time.Time

	// number of queries sent to the peer
	nQueries uint64

	// number of queries that resulted an in error
	nErrors uint64
}

func newResponseStats() *responseStats {
	return &responseStats{
		earliest: time.Unix(0, 0),
		latest:   time.Unix(0, 0),
		nQueries: 0,
		nErrors:  0,
	}
}

// toStoredResponseStats creates a storage.ResponseStats from a peer.ResponseStats.
func (rs *responseStats) toStoredResponseStats() *storage.ResponseStats {
	return &storage.ResponseStats{
		Earliest: rs.earliest.Unix(),
		Latest:   rs.latest.Unix(),
		NQueries: rs.nQueries,
		NErrors:  rs.nErrors,
	}
}
