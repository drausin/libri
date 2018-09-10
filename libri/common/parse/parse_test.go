package parse

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddr(t *testing.T) {
	cases := []struct {
		ip       string
		port     int
		expected *net.TCPAddr
	}{
		{"192.168.1.1", 20100, &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 20100}},
		{"192.168.1.1", 11001, &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 11001}},
		{"localhost", 20100, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}},
	}
	for _, c := range cases {
		actual, err := Addr(c.ip, c.port)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func TestAddrs_ok(t *testing.T) {
	addrs := []string{
		"192.168.1.1:20100",
		"192.168.1.1:11001",
		"localhost:20100",
	}
	expectedNetAddrs := []*net.TCPAddr{
		{IP: net.ParseIP("192.168.1.1"), Port: 20100},
		{IP: net.ParseIP("192.168.1.1"), Port: 11001},
		{IP: net.ParseIP("127.0.0.1"), Port: 20100},
	}
	actualNetAddrs, err := Addrs(addrs)

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
		na, err := Addrs([]string{a})
		assert.Nil(t, na, a)
		assert.NotNil(t, err, a)
	}
}

func TestParseAddrs_multi_err(t *testing.T) {
	oneBad := []string{"192.168.1.1:20100", "192.168.1.1:A"}
	as1, err := Addrs(oneBad)
	assert.Len(t, as1, 1)
	assert.Nil(t, err)

	bothBad := []string{"192.168.1.1", "192.168.1.1:A"}
	as2, err := Addrs(bothBad)
	assert.Nil(t, as2)
	assert.NotNil(t, err)
}
