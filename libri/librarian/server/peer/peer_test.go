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

	rs := p.Responses().(*responseStats)
	assert.Equal(t, uint64(0), rs.nQueries)
	assert.Equal(t, uint64(0), rs.nErrors)
	assert.Equal(t, int64(0), rs.latest.Unix())
	assert.Equal(t, int64(0), rs.earliest.Unix())
}

func TestPeer_Before(t *testing.T) {
	cases := []struct {
		prs *responseStats // for now, ResponseStats are the only thing that influence
		qrs *responseStats // peer ordering, so we just define those here
		out bool
	}{
		{
			prs: &responseStats{latest: time.Unix(0, 0)},
			qrs: &responseStats{latest: time.Unix(0, 0)},
			out: false, // before is strict
		},
		{
			prs: &responseStats{latest: time.Unix(0, 0)},
			qrs: &responseStats{latest: time.Unix(1, 0)},
			out: true,
		},
		{
			prs: &responseStats{
				earliest: time.Unix(0, 0),
				latest:   time.Unix(15, 0),
			},
			qrs: &responseStats{
				earliest: time.Unix(5, 0),
				latest:   time.Unix(10, 0),
			},
			out: false, // Earliest has no effect
		},
	}
	for _, c := range cases {
		p := &peer{resp: c.prs}
		q := &peer{resp: c.qrs}
		assert.Equal(t, c.out, p.Before(q))
	}
}
