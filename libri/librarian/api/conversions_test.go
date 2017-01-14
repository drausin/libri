package api

import (
	"math/big"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		from := &PeerAddress{Ip: c.ip, Port: uint32(c.port)}
		to := ToAddress(from)
		assert.Equal(t, c.ip, to.IP.String())
		assert.Equal(t, c.port, to.Port)
	}
}

func TestFromAddress(t *testing.T) {
	cases := []struct {
		id   *big.Int
		ip   string
		port int
	}{
		{id: big.NewInt(0), ip: "192.168.1.1", port: 1234},
		{id: big.NewInt(1), ip: "10.11.12.13", port: 100000},
		{id: big.NewInt(2), ip: "10.11.12.13", port: 1100},
	}
	for _, c := range cases {
		to := &net.TCPAddr{IP: net.ParseIP(c.ip), Port: c.port}
		from := FromAddress(c.id, to)
		assert.Equal(t, c.ip, from.Ip)
		assert.Equal(t, c.port, int(from.Port))
	}
}
