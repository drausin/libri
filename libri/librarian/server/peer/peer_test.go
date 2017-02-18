package peer

import (
	"net"
	"testing"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	id, name := cid.FromInt64(0), "test name"
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
