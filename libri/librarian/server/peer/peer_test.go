package peer

import (
	"math/big"
	"net"
	"testing"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	id, name := big.NewInt(0), "test name"
	addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1000}
	p := New(id, name, addr)
	assert.Equal(t, id, p.ID)
	assert.Equal(t, cid.String(id), p.IDStr)
	assert.Equal(t, name, p.Name)
	assert.Equal(t, addr, addr)
	assert.Equal(t, uint64(0), p.Responses.NQueries)
	assert.Equal(t, uint64(0), p.Responses.NErrors)
	assert.Equal(t, int64(0), p.Responses.Latest.Unix())
	assert.Equal(t, int64(0), p.Responses.Earliest.Unix())

}

func TestPeer_Before(t *testing.T) {
	cases := []struct {
		prs *ResponseStats // for now, ResponseStats are the only thing that influence
		qrs *ResponseStats // peer ordering, so we just define those here
		out bool
	}{
		{
			prs: &ResponseStats{Latest: time.Unix(0, 0)},
			qrs: &ResponseStats{Latest: time.Unix(0, 0)},
			out: false, // before is strict
		},
		{
			prs: &ResponseStats{Latest: time.Unix(0, 0)},
			qrs: &ResponseStats{Latest: time.Unix(1, 0)},
			out: true,
		},
		{
			prs: &ResponseStats{
				Earliest: time.Unix(0, 0),
				Latest:   time.Unix(15, 0),
			},
			qrs: &ResponseStats{
				Earliest: time.Unix(5, 0),
				Latest:   time.Unix(10, 0),
			},
			out: false, // Earliest has no effect
		},
	}
	for _, c := range cases {
		p := &Peer{Responses: c.prs}
		q := &Peer{Responses: c.qrs}
		assert.Equal(t, c.out, p.Before(q))
	}
}

func TestPeer_RecordResponseSuccess(t *testing.T) {
	p := New(big.NewInt(0), "", nil)

	p.RecordResponseSuccess()
	assert.Equal(t, uint64(1), p.Responses.NQueries)
	assert.Equal(t, uint64(0), p.Responses.NErrors)
	assert.True(t, p.Responses.Latest.Unix() > 0)
	assert.True(t, p.Responses.Earliest.Unix() > 0)
	assert.True(t, p.Responses.Latest.Equal(p.Responses.Earliest))

	p.RecordResponseSuccess()
	assert.Equal(t, uint64(2), p.Responses.NQueries)
	assert.Equal(t, uint64(0), p.Responses.NErrors)
	assert.True(t, p.Responses.Latest.Unix() > 0)
	assert.True(t, p.Responses.Earliest.Unix() > 0)
	assert.True(t, p.Responses.Latest.After(p.Responses.Earliest))
}

func TestPeer_RecordResponseError(t *testing.T) {
	p := New(big.NewInt(0), "", nil)

	p.RecordResponseError()
	assert.Equal(t, uint64(1), p.Responses.NQueries)
	assert.Equal(t, uint64(1), p.Responses.NErrors)
	assert.True(t, p.Responses.Latest.Unix() > 0)
	assert.True(t, p.Responses.Earliest.Unix() > 0)
	assert.True(t, p.Responses.Latest.Equal(p.Responses.Earliest))

	p.RecordResponseSuccess()
	assert.Equal(t, uint64(2), p.Responses.NQueries)
	assert.Equal(t, uint64(1), p.Responses.NErrors)
	assert.True(t, p.Responses.Latest.Unix() > 0)
	assert.True(t, p.Responses.Earliest.Unix() > 0)
	assert.True(t, p.Responses.Latest.After(p.Responses.Earliest))
}
