package peer

import (
	"math/big"
	"net"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	cid "github.com/drausin/libri/libri/common/id"
	"google.golang.org/grpc"
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

	// time of latest response from the peer
	Responses *ResponseStats

	// Librarian client to peer
	client *api.LibrarianClient

	// client connection to the peer
	conn *grpc.ClientConn
}

// New creates a new Peer instance with empty response stats.
func New(id *big.Int, name string, publicAddress *net.TCPAddr) *Peer {
	return &Peer{
		ID:            id,
		IDStr:         cid.String(id),
		Name:          name,
		PublicAddress: publicAddress,
		Responses:     newResponseStats(),
	}
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (p *Peer) Connect() error {
	// TODO (drausin) add SSL
	conn, err := grpc.Dial(p.PublicAddress.String(), grpc.WithInsecure())
	if err != nil {
		return err
	}
	client := api.NewLibrarianClient(conn)
	p.conn = conn
	p.client = &client
	return nil
}

// Disconnect closes the connection with the peer.
func (p *Peer) Disconnect() error {
	if p.client != nil {
		p.client = nil
		return p.conn.Close()
	}
	return nil
}

// Before returns whether p should be ordered before q in the priority queue of peers to query.
// Currently, it just uses whether p's latest response time is before q's.
func (p *Peer) Before(q *Peer) bool {
	return p.Responses.Latest.Before(q.Responses.Latest)
}

// RecordResponseSuccess records a successful response from the peer.
func (p *Peer) RecordResponseSuccess() {
	p.recordResponse()
}

// RecordResponseError records an unsuccessful or error-laden response from the peer.
func (p *Peer) RecordResponseError() {
	p.Responses.NErrors++
	p.recordResponse()
}

// recordsResponse records any response from the peer.
func (p *Peer) recordResponse() {
	p.Responses.NQueries++
	p.Responses.Latest = time.Now().UTC()
	if p.Responses.Earliest.Unix() == 0 {
		p.Responses.Earliest = p.Responses.Latest
	}
}

// ResponseStats describes metrics associated with a peer's communication history.
type ResponseStats struct {
	// earliest response time from the peer
	Earliest time.Time

	// latest response time form the peer
	Latest time.Time

	// number of queries sent to the peer
	NQueries uint64

	// number of queries that resulted an in error
	NErrors uint64
}

func newResponseStats() *ResponseStats {
	return &ResponseStats{
		Earliest: time.Unix(0, 0),
		Latest:   time.Unix(0, 0),
		NQueries: 0,
		NErrors:  0,
	}
}
