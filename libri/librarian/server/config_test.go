package server

import (
	"net"
	"testing"

	"github.com/drausin/libri/libri/common/parse"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/replicate"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	assert.NotEmpty(t, c.LocalPort)
	assert.NotEmpty(t, c.LocalMetricsPort)
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
	assert.NotEmpty(t, c.Replicate)
	assert.NotEmpty(t, c.LogLevel)
}

func TestConfig_WithLocalPort(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLocalPort()
	assert.Equal(t, c1.LocalPort, c2.WithLocalPort(0).LocalPort)
	c3Port := 1234
	assert.NotEqual(t, c1.LocalPort, c3.WithLocalPort(c3Port).LocalPort)
}

func TestConfig_WithLocalMetricsAddr(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLocalMetricsPort()
	assert.Equal(t, c1.LocalMetricsPort, c2.WithLocalMetricsPort(0).LocalMetricsPort)
	c3Port := 1234
	assert.NotEqual(t, c1.LocalMetricsPort, c3.WithLocalMetricsPort(c3Port).LocalMetricsPort)
}

func TestConfig_WithLocalProfilerPort(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLocalProfilerPort()
	assert.Equal(t, c1.LocalProfilerPort, c2.WithLocalProfilerPort(0).LocalProfilerPort)
	c3Port := 1234
	assert.NotEqual(t, c1.LocalProfilerPort, c3.WithLocalProfilerPort(c3Port).LocalProfilerPort)
}

func TestConfig_WithPublicAddr(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultPublicAddr()
	assert.Equal(t, c1.PublicAddr, c2.WithPublicAddr(nil).PublicAddr)
	c3Addr, err := parse.Addr("localhost", 1234)
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
	c3Addr, err := parse.Addr("localhost", 1234)
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

func TestConfig_WithReplicate(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultReplicate()
	assert.Equal(t, c1.Replicate, c2.WithReplicate(nil).Replicate)
	assert.NotEqual(t,
		c1.Replicate,
		c3.WithReplicate(&replicate.Parameters{VerifyInterval: 0}).Replicate,
	)
}

func TestConfig_WithReportMetrics(t *testing.T) {
	c1, c2, c3 := NewDefaultConfig(), NewDefaultConfig(), NewDefaultConfig()
	c1.WithDefaultReportMetrics()
	assert.True(t, c1.ReportMetrics)
	assert.True(t, c1.Replicate.ReportMetrics)
	c2.WithReportMetrics(true)
	assert.True(t, c2.ReportMetrics)
	assert.True(t, c2.Replicate.ReportMetrics)
	c3.WithReportMetrics(false)
	assert.False(t, c3.ReportMetrics)
	assert.False(t, c3.Replicate.ReportMetrics)
}

func TestConfig_WithProfile(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultProfile()
	assert.False(t, c1.Profile)
	c2.WithProfile(true)
	assert.True(t, c2.Profile)
	c3.WithProfile(false)
	assert.False(t, c3.Profile)
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

	addr, err := parse.Addr("localhost", DefaultPort+1)
	assert.Nil(t, err)
	config.WithPublicAddr(addr)
	assert.False(t, config.isBootstrap())
}
