package server

import (
	"net"
	"testing"

	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	assert.NotEmpty(t, c.LocalAddr)
	assert.NotEmpty(t, c.PublicAddr)
	assert.NotEmpty(t, c.PublicName)
	assert.NotEmpty(t, c.DataDir)
	assert.NotEmpty(t, c.DbDir)
	assert.NotEmpty(t, c.BootstrapAddrs)
	assert.NotEmpty(t, c.Routing)
	assert.NotEmpty(t, c.Introduce)
	assert.NotEmpty(t, c.Search)
	assert.NotEmpty(t, c.Store)
	assert.NotEmpty(t, c.SubscribeTo)
	assert.NotEmpty(t, c.SubscribeFrom)
	assert.NotEmpty(t, c.LogLevel)
}

func TestConfig_WithLocalAddr(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLocalAddr()
	assert.Equal(t, c1.LocalAddr, c2.WithLocalAddr(nil).LocalAddr)
	c3Addr, err := ParseAddr("localhost", 1234)
	assert.Nil(t, err)
	assert.NotEqual(t, c1.LocalAddr, c3.WithLocalAddr(c3Addr).LocalAddr)
}

func TestConfig_WithPublicAddr(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultPublicAddr()
	assert.Equal(t, c1.PublicAddr, c2.WithPublicAddr(nil).PublicAddr)
	c3Addr, err := ParseAddr("localhost", 1234)
	assert.Nil(t, err)
	assert.NotEqual(t,
		c1.PublicAddr,
		c3.WithPublicAddr(c3Addr).PublicAddr,
	)
}

func TestConfig_WithPublicName(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultPublicName()
	assert.Equal(t, c1.PublicName, c2.WithPublicName("").PublicName)
	assert.NotEqual(t, c1.PublicName, c3.WithPublicName("some other name").PublicName)
}

func TestConfig_WithDataDir(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultDataDir()
	assert.Equal(t, c1.DataDir, c2.WithDataDir("").DataDir)
	assert.NotEqual(t, c1.DataDir, c3.WithDataDir("/some/other/dir").DataDir)
}

func TestConfig_WithDBDir(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultDBDir()
	assert.Equal(t, c1.DbDir, c2.WithDBDir("").DbDir)
	assert.NotEqual(t, c1.DbDir, c3.WithDBDir("/some/other/dir").DbDir)
}

func TestConfig_WithBootstrapAddrs(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultBootstrapAddrs()
	assert.Equal(t, c1.BootstrapAddrs, c2.WithBootstrapAddrs(nil).BootstrapAddrs)
	c3Addr, err := ParseAddr("localhost", 1234)
	assert.Nil(t, err)
	assert.NotEqual(t,
		c1.BootstrapAddrs,
		c3.WithBootstrapAddrs([]*net.TCPAddr{c3Addr}).BootstrapAddrs,
	)
}

func TestConfig_WithRouting(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultRouting()
	assert.Equal(t, c1.Routing, c2.WithRouting(nil).Routing)
	assert.NotEqual(t,
		c1.Routing,
		c3.WithRouting(&routing.Parameters{MaxBucketPeers: 10}).Routing,
	)
}

func TestConfig_WithIntroduce(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultIntroduce()
	assert.Equal(t, c1.Introduce, c2.WithIntroduce(nil).Introduce)
	assert.NotEqual(t,
		c1.Introduce,
		c3.WithIntroduce(&introduce.Parameters{Concurrency: 1}).Introduce,
	)
}

func TestConfig_WithSearch(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultSearch()
	assert.Equal(t, c1.Search, c2.WithSearch(nil).Search)
	assert.NotEqual(t,
		c1.Search,
		c3.WithSearch(&search.Parameters{Concurrency: 1}).Search,
	)
}

func TestConfig_WithStore(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultStore()
	assert.Equal(t, c1.Store, c2.WithStore(nil).Store)
	assert.NotEqual(t,
		c1.Store,
		c3.WithStore(&store.Parameters{Concurrency: 1}).Store,
	)
}

func TestConfig_WithSubscribeTo(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultSubscribeTo()
	assert.Equal(t, c1.SubscribeTo, c2.WithSubscribeTo(nil).SubscribeTo)
	assert.NotEqual(t,
		c1.SubscribeTo,
		c3.WithSubscribeTo(&subscribe.ToParameters{NSubscriptions: 3}).SubscribeTo,
	)
}

func TestConfig_WithSubscribeFrom(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultSubscribeFrom()
	assert.Equal(t, c1.SubscribeFrom, c2.WithSubscribeFrom(nil).SubscribeFrom)
	assert.NotEqual(t,
		c1.SubscribeFrom,
		c3.WithSubscribeFrom(&subscribe.FromParameters{NMaxSubscriptions: 0}).SubscribeFrom,
	)
}

func TestConfig_WithLogLevel(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLogLevel()
	assert.Equal(t, c1.LogLevel, c2.WithLogLevel(0).LogLevel)
	assert.NotEqual(t,
		c1.LogLevel,
		c3.WithLogLevel(zapcore.DebugLevel).LogLevel,
	)
}

func TestConfig_isBootstrap(t *testing.T) {
	config := NewDefaultConfig()
	assert.True(t, config.isBootstrap())

	addr, err := ParseAddr("localhost", DefaultPort+1)
	assert.Nil(t, err)
	config.WithPublicAddr(addr)
	assert.False(t, config.isBootstrap())
}

func TestParseAddr(t *testing.T) {
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
		actual, err := ParseAddr(c.ip, c.port)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func TestParseAddrs_ok(t *testing.T) {
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
