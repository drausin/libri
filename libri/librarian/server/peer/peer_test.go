package peer

import (
	"math/rand"
	"net"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	peerID, name := id.FromInt64(1), "test name"
	addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1000}
	p := New(peerID, name, addr)
	assert.Equal(t, 0, peerID.Cmp(p.ID()))
	assert.Equal(t, name, p.(*peer).name)
	assert.Equal(t, addr, addr)
}

func TestNewStub(t *testing.T) {
	peerID := id.FromInt64(1)
	name := "some name"
	p := NewStub(peerID, name)
	assert.Equal(t, peerID, p.ID())
	assert.Equal(t, name, p.(*peer).name)
	assert.Nil(t, p.Address())
}

func TestPeer_Merge_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	var p1, p2 Peer

	// p2's name should replace p1's, and response counts should sum
	p1ID := id.NewPseudoRandom(rng)
	p1 = New(p1ID, "p1", &net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 20100,
	})

	p2Name := "p2"
	p2 = New(p1.ID(), p2Name, &net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 20100,
	})

	err := p1.Merge(p2)
	assert.Nil(t, err)
	assert.Equal(t, p1.(*peer).name, p2Name)

	// p2's empty name should not replace p1's
	p1 = NewTestPeer(rng, 0)
	p1Name := p1.(*peer).name
	p2 = New(p1.ID(), "", p1.Address())
	err = p1.Merge(p2)
	assert.Nil(t, err)
	assert.Equal(t, p1Name, p1.(*peer).name)

	// p2's connector should replace p1's
	p1ID = id.NewPseudoRandom(rng)
	p1 = New(p1ID, "p1", &net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 20100,
	})
	p2Conn := &net.TCPAddr{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 11001,
	}
	p2 = New(p1ID, "p1", p2Conn)
	err = p1.Merge(p2)
	assert.Nil(t, err)
	assert.Equal(t, p2Conn, p1.Address())
}

func TestPeer_Merge_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	var p1, p2 Peer

	// can't merge p2 into p1 b/c has different ID
	p1, p2 = NewTestPeer(rng, 0), NewTestPeer(rng, 1)
	err := p1.Merge(p2)
	assert.NotNil(t, err)
}

func TestPeer_ToAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p := NewTestPeer(rng, 0)
	apiP := p.ToAPI()

	assert.Equal(t, p.ID().Bytes(), apiP.PeerId)
	assert.Equal(t, p.Address().IP.String(), apiP.Ip)
	assert.Equal(t, uint32(p.Address().Port), apiP.Port)
}

func TestFromer_FromAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p1 := NewTestPeer(rng, 0)
	f := NewFromer()
	apiP := p1.ToAPI()
	p2 := f.FromAPI(apiP)

	assert.Equal(t, p1.ID(), p2.ID())
	assert.Equal(t, p1.Address(), p2.Address())
}

func TestToAPIs(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	ns := []int{0, 1, 2, 4}
	for i := 0; i < len(ns); i++ {
		assert.Equal(t, ns[i], len(ToAPIs(NewTestPeers(rng, ns[i]))))
	}
}

func TestToAddress(t *testing.T) {
	cases := []struct {
		ip   string
		port int
	}{
		{ip: "192.168.1.1", port: 1234},
		{ip: "10.11.12.13", port: 100000},
		{ip: "10.11.12.13", port: 1100},
	}
	for _, c := range cases {
		from := &api.PeerAddress{Ip: c.ip, Port: uint32(c.port)}
		to := ToAddress(from)
		assert.Equal(t, c.ip, to.IP.String())
		assert.Equal(t, c.port, to.Port)
	}
}

func TestFromAddress(t *testing.T) {
	cases := []struct {
		id   id.ID
		name string
		ip   string
		port int
	}{
		{id: id.FromInt64(0), name: "peer-0", ip: "192.168.1.1", port: 1234},
		{id: id.FromInt64(1), name: "peer-1", ip: "10.11.12.13", port: 100000},
		{id: id.FromInt64(2), name: "", ip: "10.11.12.13", port: 1100},
	}
	for _, c := range cases {
		to := &net.TCPAddr{IP: net.ParseIP(c.ip), Port: c.port}
		from := FromAddress(c.id, c.name, to)
		assert.Equal(t, c.ip, from.Ip)
		assert.Equal(t, c.name, from.PeerName)
		assert.Equal(t, c.port, int(from.Port))
	}
}
