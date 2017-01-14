package peer

import (
	"math/big"
	"net"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"google.golang.org/grpc"
)

// Peer represents a peer in the network.
type Peer interface {

	// ID returns the peer ID.
	ID() *big.Int

	// IDStr returns the string encoding of the peer ID.
	IDStr() string

	// Name returns the self-reported name.
	Name() string

	// PublicAddress returns the peer's public address.
	PublicAddress() *net.TCPAddr

	// ResponseStats returns the peer's response stats.
	ResponseStats() *responseStats

	// Before returns whether p should be ordered before q in the priority queue of peers to
	// query. Currently, it just uses whether p's latest response time is before q's.
	Before(Peer) bool

	// RecordResponseSuccess records a successful response from the peer.
	RecordResponseSuccess()

	// RecordResponseError records an unsuccessful or error-laden response from the peer.
	RecordResponseError()

	// Connect establishes the TCP connection with the peer and establishes the Librarian
	// client with it.
	Connect() error

	// Disconnect closes the connection with the peer.
	Disconnect() error

	// ToStored returns a storage.Peer versin of the peer.
	ToStored() *storage.Peer
}

type peer struct {
	// 256-bit ID
	id *big.Int

	// string encoding of ID
	idStr string

	// self-reported name
	name string

	// RPC TCP address
	publicAddress *net.TCPAddr

	// time of latest response from the peer
	responses *responseStats

	// Librarian client to peer
	client *api.LibrarianClient

	// client connection to the peer
	conn *grpc.ClientConn
}

// New creates a new Peer instance with empty response stats.
func New(id *big.Int, name string, publicAddress *net.TCPAddr) Peer {
	return NewWithResponseStats(id, name, publicAddress, newResponseStats())
}

// NewWithResponseStats creates a new Peer instance with the given response stats.
func NewWithResponseStats(id *big.Int, name string, publicAddress *net.TCPAddr,
	rs *responseStats) Peer {
	return &peer{
		id:            id,
		idStr:         cid.String(id),
		name:          name,
		publicAddress: publicAddress,
		responses:     rs,
	}
}

func (p *peer) ID() *big.Int {
	return p.id
}

func (p *peer) IDStr() string {
	return p.idStr
}

func (p *peer) Name() string {
	return p.name
}

func (p *peer) PublicAddress() *net.TCPAddr {
	return p.publicAddress
}

func (p *peer) ResponseStats() *responseStats {
	return p.responses
}

// Before returns whether p should be ordered before q in the priority queue of peers to query.
// Currently, it just uses whether p's latest response time is before q's.
func (p *peer) Before(q Peer) bool {
	return p.ResponseStats().latest.Before(q.ResponseStats().latest)
}

// RecordResponseSuccess records a successful response from the peer.
func (p *peer) RecordResponseSuccess() {
	p.recordResponse()
}

// RecordResponseError records an unsuccessful or error-laden response from the peer.
func (p *peer) RecordResponseError() {
	p.responses.nErrors++
	p.recordResponse()
}

// Connect establishes the TCP connection with the peer and establishes the Librarian client with
// it.
func (p *peer) Connect() error {
	// TODO (drausin) add SSL
	conn, err := grpc.Dial(p.publicAddress.String(), grpc.WithInsecure())
	if err != nil {
		return err
	}
	client := api.NewLibrarianClient(conn)
	p.conn = conn
	p.client = &client
	return nil
}

// Disconnect closes the connection with the peer.
func (p *peer) Disconnect() error {
	if p.client != nil {
		p.client = nil
		return p.conn.Close()
	}
	return nil
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
