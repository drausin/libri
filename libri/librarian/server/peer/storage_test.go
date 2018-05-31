package peer

import (
	"math/rand"
	"net"
	"testing"

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
