package author

import (
	"testing"
	"github.com/drausin/libri/libri/author/io/print"
	"github.com/stretchr/testify/assert"
	"net"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/author/io/publish"
	"go.uber.org/zap/zapcore"
)

func TestNewDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	assert.NotEmpty(t, c.DataDir)
	assert.NotEmpty(t, c.DbDir)
	assert.NotEmpty(t, c.KeychainDir)
	assert.NotEmpty(t, c.LibrarianAddrs)
	assert.NotEmpty(t, c.Print)
	assert.NotEmpty(t, c.Publish)
	assert.NotEmpty(t, c.LogLevel)
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

func TestConfig_WithKeychainDir(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultKeychainDir()
	assert.Equal(t, c1.KeychainDir, c2.WithKeychainDir("").KeychainDir)
	assert.NotEqual(t, c1.KeychainDir, c3.WithKeychainDir("/some/other/dir").KeychainDir)
}

func TestConfig_WithBootstrapAddrs(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultLibrarianAddrs()
	assert.Equal(t, c1.LibrarianAddrs, c2.WithLibrarianAddrs(nil).LibrarianAddrs)
	assert.NotEqual(t,
		c1.LibrarianAddrs,
		c3.WithLibrarianAddrs(
			[]*net.TCPAddr{server.ParseAddr("localhost", 1234)},
		).LibrarianAddrs,
	)
}

func TestConfig_WithPrint(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultPrint()
	assert.Equal(t, c1.Print, c2.WithPrint(nil).Print)
	assert.NotEqual(t,
		c1.Print,
		c3.WithPrint(&print.Parameters{Parallelism: 1}).Print,
	)
}

func TestConfig_WithPublish(t *testing.T) {
	c1, c2, c3 := &Config{}, &Config{}, &Config{}
	c1.WithDefaultPublish()
	assert.Equal(t, c1.Print, c2.WithPublish(nil).Print)
	assert.NotEqual(t,
		c1.Publish,
		c3.WithPublish(&publish.Parameters{PutParallelism: 1}).Publish,
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
