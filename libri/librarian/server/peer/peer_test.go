package peer

import (
	"net"
	"testing"
	"time"
	"math/rand"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	id, name := cid.FromInt64(1), "test name"
	addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1000}
	p := New(id, name, NewConnector(addr))
	assert.Equal(t, 0, id.Cmp(p.ID()))
	assert.Equal(t, name, p.(*peer).name)
	assert.Equal(t, addr, addr)

	rs := p.Recorder().(*queryRecorder)
	assert.Equal(t, uint64(0), rs.responses.nQueries)
	assert.Equal(t, uint64(0), rs.responses.nErrors)
	assert.Equal(t, int64(0), rs.responses.latest.Unix())
	assert.Equal(t, int64(0), rs.responses.earliest.Unix())
}

func TestNewStub(t *testing.T) {
	peerID := cid.FromInt64(1)
	p := NewStub(peerID)
	assert.Equal(t, peerID, p.ID())
	assert.Nil(t, p.Connector())
}

func TestPeer_Before(t *testing.T) {
	cases := map[string]struct {
		pqr    *queryRecorder // for now, query records are the only thing that influence
		qqr    *queryRecorder // peer ordering, so we just define those here
		before bool
	}{
		"strict": {
			pqr: &queryRecorder{
				responses: &queryTypeOutcomes{latest: time.Unix(0, 0)},
			},
			qqr: &queryRecorder{
				responses: &queryTypeOutcomes{latest: time.Unix(0, 0)},
			},
			before: false, // before is strict
		},
		"standard": {
			pqr: &queryRecorder{
				responses: &queryTypeOutcomes{latest: time.Unix(0, 0)},
			},
			qqr: &queryRecorder{
				responses: &queryTypeOutcomes{latest: time.Unix(1, 0)},
			},
			before: true,
		},
		"earliest": {
			pqr: &queryRecorder{
				responses: &queryTypeOutcomes{
					earliest: time.Unix(0, 0),
					latest:   time.Unix(15, 0),
				},
			},
			qqr: &queryRecorder{
				responses: &queryTypeOutcomes{
					earliest: time.Unix(5, 0),
					latest:   time.Unix(10, 0),
				},
			},
			before: false, // Earliest has no effect
		},
	}
	for label, c := range cases {
		p := &peer{recorder: c.pqr}
		q := &peer{recorder: c.qqr}
		assert.Equal(t, c.before, p.Before(q), label)
	}
}

func TestPeer_Merge_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	var p1, p2 Peer

	// p2's name should replace p1's, and response counts should sum
	p1ID := cid.NewPseudoRandom(rng)
	p1 = New(p1ID, "p1", NewConnector(&net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 11000,
	}))
	p1.Recorder().Record(Request, Success)
	assert.Equal(t, uint64(1), p1.Recorder().(*queryRecorder).requests.nQueries)
	assert.Equal(t, uint64(0), p1.Recorder().(*queryRecorder).responses.nQueries)

	p2Name := "p2"
	p2 = New(p1.ID(), p2Name, p1.Connector())
	p2.Recorder().Record(Request, Success)
	p2.Recorder().Record(Response, Success)
	assert.Equal(t, uint64(1), p2.Recorder().(*queryRecorder).requests.nQueries)
	assert.Equal(t, uint64(1), p2.Recorder().(*queryRecorder).responses.nQueries)

	err := p1.Merge(p2)
	assert.Nil(t, err)
	assert.Equal(t, p1.(*peer).name, p2Name)
	assert.Equal(t, uint64(2), p1.Recorder().(*queryRecorder).requests.nQueries)
	assert.Equal(t, uint64(1), p1.Recorder().(*queryRecorder).responses.nQueries)

	// p2's empty name should not replace p1's
	p1 = NewTestPeer(rng, 0)
	p1Name := p1.(*peer).name
	p2 = New(p1.ID(), "", p1.Connector())
	err = p1.Merge(p2)
	assert.Nil(t, err)
	assert.Equal(t, p1Name, p1.(*peer).name)
}

func TestPeer_Merge_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	var p1, p2 Peer

	// can't merge p2 into p1 b/c has different ID
	p1, p2 = NewTestPeer(rng, 0), NewTestPeer(rng, 1)
	err := p1.Merge(p2)
	assert.NotNil(t, err)

	// can't merge p2 into p1 b/c p2's connector has different address
	p1ID := cid.NewPseudoRandom(rng)
	p1 = New(p1ID, "p1", NewConnector(&net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 11000,
	}))
	p2 = New(p1ID, "", NewConnector(&net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 11001,
	}))
	err = p1.Merge(p2)
	assert.NotNil(t, err)

}

func TestPeer_ToAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p := NewTestPeer(rng, 0)
	apiP := p.ToAPI()

	assert.Equal(t, p.ID().Bytes(), apiP.PeerId)
	assert.Equal(t, p.Connector().(*connector).publicAddress.IP.String(), apiP.Ip)
	assert.Equal(t, uint32(p.Connector().(*connector).publicAddress.Port), apiP.Port)
}

func TestFromer_FromAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p1 := NewTestPeer(rng, 0)
	f := NewFromer()
	apiP := p1.ToAPI()
	p2 := f.FromAPI(apiP)

	assert.Equal(t, p1.ID(), p2.ID())
	assert.Equal(t, p1.Connector().(*connector).publicAddress,
		p2.Connector().(*connector).publicAddress)
}
