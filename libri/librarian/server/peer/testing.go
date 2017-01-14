package peer

import (
	"fmt"
	"log"
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
func NewTestPeer(rng *rand.Rand, idx int) *Peer {
	id := cid.NewPseudoRandom(rng)
	address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("192.168.1.1:%v", 11000+idx))
	if err != nil {
		panic(err)
	}
	p := New(id, fmt.Sprintf("peer-%d", idx+1), address)
	now := time.Unix(int64(idx), 0).UTC()
	p.Responses = &ResponseStats{
		Earliest: now,
		Latest:   now,
		NQueries: 1,
		NErrors:  0,
	}
	return p
}

// NewTestPeers generates n new peers suitable for testing use with random IDs and incrementing
// values of other fields.
func NewTestPeers(rng *rand.Rand, n int) []*Peer {
	ps := make([]*Peer, n)
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
		Responses: &storage.ResponseStats{
			Earliest: now.Unix(),
			Latest:   now.Unix(),
			NQueries: 1,
			NErrors:  0,
		},
	}
}

// AssertPeersEqual checks that the stored and non-stored representations of a peer are equal.
func AssertPeersEqual(t *testing.T, sp *storage.Peer, p *Peer) {
	log.Printf("sp: %v, p: %v", sp, p)
	assert.Equal(t, sp.Id, p.ID.Bytes())
	assert.Equal(t, sp.Name, p.Name)
	assert.Equal(t, sp.PublicAddress.Ip, p.PublicAddress.IP.String())
	assert.Equal(t, sp.PublicAddress.Port, uint32(p.PublicAddress.Port))
	assert.Equal(t, sp.Responses.Earliest, p.Responses.Earliest.Unix())
	assert.Equal(t, sp.Responses.Latest, p.Responses.Latest.Unix())
	assert.Equal(t, sp.Responses.NQueries, p.Responses.NQueries)
	assert.Equal(t, sp.Responses.NErrors, p.Responses.NErrors)
}
