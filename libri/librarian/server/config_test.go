package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	assert.NotEmpty(t, c.LocalAddr)
	assert.NotEmpty(t, c.PublicAddr)
	assert.NotEmpty(t, c.LocalName)
	assert.NotEmpty(t, c.PublicName)
	assert.NotEmpty(t, c.DataDir)
	assert.NotEmpty(t, c.DbDir)
	assert.NotEmpty(t, c.BootstrapAddrs)
	assert.NotEmpty(t, c.LogLevel)
}

func TestConfig_isBootstrap(t *testing.T) {
	config := DefaultConfig()
	assert.True(t, config.isBootstrap())

	config.WithPublicAddr(ParseAddr("localhost", DefaultPort+1))
	assert.False(t, config.isBootstrap())
}

func TestParseAddr(t *testing.T) {
	cases := []struct {
		ip      string
		port    int
		netAddr *net.TCPAddr
	}{
		{"192.168.1.1", 11000, &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 11000}},
		{"192.168.1.1", 11001, &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 11001}},
		{"localhost", 11000, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11000}},
	}
	for _, c := range cases {
		assert.Equal(t, c.netAddr, ParseAddr(c.ip, c.port))
	}
}

func TestParseAddrs_ok(t *testing.T) {
	addrs := []string{
		"192.168.1.1:11000",
		"192.168.1.1:11001",
		"localhost:11000",
	}
	expectedNetAddrs := []*net.TCPAddr{
		{IP: net.ParseIP("192.168.1.1"), Port: 11000},
		{IP: net.ParseIP("192.168.1.1"), Port: 11001},
		{IP: net.ParseIP("127.0.0.1"), Port: 11000},
	}
	actualNetAddrs, err := ParseAddrs(addrs)

	assert.Nil(t, err)
	for i, a := range actualNetAddrs {
		assert.Equal(t, expectedNetAddrs[i], a)
	}
}

func TestParseAddrs_err(t *testing.T) {
	addrs := []string{
		"192.168.1.1",         // no port
		"192.168.1.1:A",       // bad port
		"192::168::1:1:11001", // IPv6 instead of IPv4
		"192.168.1.1.11001",   // bad port delimiter
	}

	// test individually
	for _, a := range addrs {
		na, err := ParseAddrs([]string{a})
		assert.Nil(t, na, a)
		assert.NotNil(t, err, a)
	}
}
