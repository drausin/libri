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
	p.RecordResponseSuccess()
	p.RecordResponseSuccess()
	p.RecordResponseError()
	sp := ToStored(p)
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

func TestFromStoredResponseStats(t *testing.T) {
	now, nQueries, nErrors := time.Now().Unix(), uint64(2), uint64(1)
	from := &storage.ResponseStats{Earliest: now, Latest: now, NQueries: nQueries, NErrors: nErrors}
	to := fromStoredResponseStats(from)
	assert.Equal(t, time.Unix(now, 0).UTC(), to.Earliest)
	assert.Equal(t, time.Unix(now, 0).UTC(), to.Latest)
	assert.Equal(t, nQueries, to.NQueries)
	assert.Equal(t, nErrors, to.NErrors)
}

func TestToStoredResponseStats(t *testing.T) {
	now, nQueries, nErrors := time.Now().UTC(), uint64(2), uint64(1)
	to := &ResponseStats{Earliest: now, Latest: now, NQueries: nQueries, NErrors: nErrors}
	from := toStoredResponseStats(to)
	assert.Equal(t, now.Unix(), from.Earliest)
	assert.Equal(t, now.Unix(), from.Latest)
	assert.Equal(t, nQueries, from.NQueries)
	assert.Equal(t, nErrors, from.NErrors)
}
