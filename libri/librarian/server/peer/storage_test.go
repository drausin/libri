package peer

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/stretchr/testify/assert"
)

func TestFromStored(t *testing.T) {
	sp := NewTestStoredPeer(rand.New(rand.NewSource(0)), 0)
	p := FromStored(sp)
	AssertPeersEqual(t, sp, p)
}

func TestToStored(t *testing.T) {
	p := NewTestPeer(rand.New(rand.NewSource(0)), 0)
	p.Recorder().Record(Response, Success)
	p.Recorder().Record(Response, Success)
	p.Recorder().Record(Response, Error)
	sp := p.ToStored()
	AssertPeersEqual(t, sp, p)
}

func TestFromStoredAddress(t *testing.T) {
	ip, port := "192.168.1.1", uint32(1000)
	sa := &storage.Address{Ip: ip, Port: port}
	a := fromStoredAddress(sa)
	assert.Equal(t, ip, a.IP.String())
	assert.Equal(t, int(port), a.Port)
}

func TestToStoredAddress(t *testing.T) {
	ip, port := "192.168.1.1", 1000
	a := &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
	sa := toStoredAddress(a)
	assert.Equal(t, ip, sa.Ip)
	assert.Equal(t, uint32(port), sa.Port)
}

func TestFromStoredQueryOutcomes(t *testing.T) {
	now, nQueries, nErrors := time.Now().Unix(), uint64(2), uint64(1)
	from := &storage.QueryOutcomes{
		Responses: &storage.QueryTypeOutcomes{
			Earliest: now,
			Latest: now,
			NQueries: nQueries,
			NErrors: nErrors,
		},
		Requests: &storage.QueryTypeOutcomes{},  // all zeros
	}
	to := fromStoredQueryOutcomes(from)
	assert.Equal(t, time.Unix(now, 0).UTC(), to.responses.earliest)
	assert.Equal(t, time.Unix(now, 0).UTC(), to.responses.latest)
	assert.Equal(t, nQueries, to.responses.nQueries)
	assert.Equal(t, nErrors, to.responses.nErrors)
}

func TestToStoredQueryOutcomes(t *testing.T) {
	now, nQueries, nErrors := time.Now().UTC(), uint64(2), uint64(1)
	qr := &queryRecorder{
		responses: &queryTypeOutcomes{
			earliest: now,
			latest: now,
			nQueries: nQueries,
			nErrors: nErrors,
		},
		requests: &queryTypeOutcomes{},  // all zeros
	}
	sqr := qr.ToStored()
	assert.Equal(t, now.Unix(), sqr.Responses.Earliest)
	assert.Equal(t, now.Unix(), sqr.Responses.Latest)
	assert.Equal(t, nQueries, sqr.Responses.NQueries)
	assert.Equal(t, nErrors, sqr.Responses.NErrors)
}
