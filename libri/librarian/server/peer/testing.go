package peer

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/stretchr/testify/assert"
)

// NewTestPeer generates a new peer suitable for testing using a random number generator for the
// ID and an index.
func NewTestPeer(rng *rand.Rand, idx int) Peer {
	id := cid.NewPseudoRandom(rng)
	address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("192.168.1.1:%v", 11000+idx))
	if err != nil {
		panic(err)
	}

	// create new recorder with a distinct response time
	now := time.Unix(int64(idx), 0).UTC()
	recorder := newQueryRecorder()
	recorder.Record(Response, Success)
	recorder.responses.latest = now
	recorder.responses.earliest = now

	conn := NewConnector(address)
	return New(id, fmt.Sprintf("peer-%d", idx+1), conn).(*peer).WithQueryRecorder(recorder)
}

// NewTestPeers generates n new peers suitable for testing use with random IDs and incrementing
// values of other fields.
func NewTestPeers(rng *rand.Rand, n int) []Peer {
	ps := make([]Peer, n)
	for i := 0; i < n; i++ {
		ps[i] = NewTestPeer(rng, i)
	}
	return ps
}

// NewTestStoredPeer generates a new storage.Peer suitable for testing using a random number
// generator for the ID and an index.
func NewTestStoredPeer(rng *rand.Rand, idx int) *storage.Peer {
	now := time.Unix(int64(idx), 0).UTC()
	return &storage.Peer{
		Id:   cid.NewPseudoRandom(rng).Bytes(),
		Name: fmt.Sprintf("peer-%d", idx+1),
		PublicAddress: &storage.Address{
			Ip:   "192.168.1.1",
			Port: uint32(11000 + idx),
		},
		QueryOutcomes: &storage.QueryOutcomes{
			Responses: &storage.QueryTypeOutcomes{
				Earliest: now.Unix(),
				Latest:   now.Unix(),
				NQueries: 1,
				NErrors:  0,
			},
			Requests: &storage.QueryTypeOutcomes{},  // everything will be zero
		},
	}
}

// AssertPeersEqual checks that the stored and non-stored representations of a peer are equal.
func AssertPeersEqual(t *testing.T, sp *storage.Peer, p Peer) {
	assert.Equal(t, sp.Id, p.ID().Bytes())
	publicAddres := p.(*peer).conn.(*connector).publicAddress
	assert.Equal(t, sp.PublicAddress.Ip, publicAddres.IP.String())
	assert.Equal(t, sp.PublicAddress.Port, uint32(publicAddres.Port))

	prs := p.Recorder().(*queryRecorder).responses
	assert.Equal(t, sp.QueryOutcomes.Responses.Earliest, prs.earliest.Unix())
	assert.Equal(t, sp.QueryOutcomes.Responses.Latest, prs.latest.Unix())
	assert.Equal(t, sp.QueryOutcomes.Responses.NQueries, prs.nQueries)
	assert.Equal(t, sp.QueryOutcomes.Responses.NErrors, prs.nErrors)
}
