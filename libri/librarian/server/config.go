package server

import (
	"crypto/md5"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/pkg/errors"
)

const (

	// DefaultPort is the default port of both local and public addresses.
	DefaultPort       = 11000

	// DefaultIP is the default IP of both local and public addresses.
	DefaultIP         = "localhost"

	// DefaultDBSubdir is the default DB subdirectory within the data dir.
	DefaultDbSubDir   = "db"

	// DefaultLogLevel is the default log level to use.
	DefaultLogLevel   = zap.InfoLevel

	// DataSubdir is the name of the data directory.
	DataSubdir = "data"

)

// Config is used to configure a Librarian server
type Config struct {
	// LocalAddr is the local address the server listens to.
	LocalAddr *net.TCPAddr

	// PublicAddr is the public address clients make requests to.
	PublicAddr *net.TCPAddr

	// LocalName is the peer name on the particular box.
	LocalName string

	// PublicName is the public facing name of the peer.
	PublicName string

	// DataDir is the local directory to store the node states.
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// BootstrapAddrs
	BootstrapAddrs []*net.TCPAddr

	// LogLevel is the log level
	LogLevel zapcore.Level
}

// DefaultConfig returns a reasonable default server configuration.
func DefaultConfig() *Config {
	config := &Config{}

	// set defaults via zero values; in cases where the config B depends on config A, config A
	// should be set before config B
	config.WithDefaultLocalAddr()
	config.WithPublicAddr(nil)
	config.WithLocalName("")
	config.WithPublicName("")
	config.WithDataDir("")
	config.WithDBDir("")
	config.WithBootstrapAddrs(nil)
	config.WithLogLevel(DefaultLogLevel)

	return config
}

// WithLocalAddr sets config's local address to the given value or to the default if the given
// value is nil.
func (c *Config) WithLocalAddr(localAddr *net.TCPAddr) *Config {
	if localAddr == nil {
		localAddr = ParseAddr(DefaultIP,  DefaultPort)
	}
	c.LocalAddr = localAddr
	return c
}

// WithDefaultLocalAddr sets the local address to the default value.
func (c *Config) WithDefaultLocalAddr() *Config {
	c.WithLocalAddr(nil)
	return c
}

// WithPublicAddr sets the public address to the given value or to the default if the given value
// is nil.
func (c *Config) WithPublicAddr(publicAddr *net.TCPAddr) *Config {
	if publicAddr == nil {
		publicAddr = c.LocalAddr
	}
	c.PublicAddr = publicAddr
	return c
}

// WithDefaultPublicAddr sets the public address to the default value.
func (c *Config) WithDefaultPublicAddr() *Config {
	c.WithPublicAddr(nil)
	return c
}

// WithLocalName sets the local name to the given value or the default if the given value is empty.
func (c *Config) WithLocalName(localName string) *Config {
	if localName == "" {
		localName = nameFromAddr(c.LocalAddr)
	}
	c.LocalName = localName
	return c
}

// WithDefaultLocalName sets the local name to the default value, comprised of a hash of the
// local address.
func (c *Config) WithDefaultLocalName() *Config {
	c.WithLocalName("")
	return c
}

// WithPublicName sets the public name to the given value or the default if the given value is
// empty.
func (c *Config) WithPublicName(publicName string) *Config {
	if publicName == "" {
		publicName = nameFromAddr(c.PublicAddr)
	}
	c.PublicName = publicName
	return c
}

// WithDefaultPublicName sets the public name to the default value, comprised of a hash of the
// public address.
func (c *Config) WithDefaultPublicName() *Config {
	c.WithPublicName("")
	return c
}

// WithDataDir sets the data dir to the given value or the default if the given value is empty.
func (c *Config) WithDataDir(dataDir string) *Config {
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	c.DataDir = dataDir
	return c
}

// WithDefaultDataDir sets the data dir to the default value.
func (c *Config) WithDefaultDataDir() *Config {
	c.WithDataDir("")
	return c
}

// WithDBDir sets the DB dir to the given value or the default if the given value is empty.
func (c *Config) WithDBDir(dbDir string) *Config {
	if dbDir == "" {
		dbDir = defaultDBDir(c.DataDir, c.LocalName, DefaultDbSubDir)
	}
	c.DbDir = dbDir
	return c
}

// WithDefaultDBDir sets the DB dir to the default value, comprised of a hash of the local address.
func (c *Config) WithDefaultDBDir() *Config {
	c.WithDBDir("")
	return c
}

// WithBootstrapAddrs sets the bootstrap addresses to the given value or the default if the given
// value is empty.
func (c *Config) WithBootstrapAddrs(bootstrapAddrs []*net.TCPAddr) *Config {
	if bootstrapAddrs == nil {
		// default is itself
		bootstrapAddrs = []*net.TCPAddr{ParseAddr(DefaultIP,  DefaultPort)}
	}
	c.BootstrapAddrs = bootstrapAddrs
	return c
}

// WithDefaultBootstrapAddrs sets the bootstrap addresses to the default value.
func (c *Config) WithDefaultBootstrapAddrs() *Config {
	c.WithBootstrapAddrs(nil)
	return c
}

// WithLogLevel sets the log level to the given value, though this doesn't have any direct effect
// on the creation of the logger instance.
func (c *Config) WithLogLevel(logLevel zapcore.Level) *Config {
	if logLevel == 0 {
		logLevel = DefaultLogLevel
	}
	c.LogLevel = logLevel
	return c
}

// WithDefaultLogLevel sets the default log level.
func (c *Config) WithDefaultLogLevel() *Config {
	c.WithLogLevel(DefaultLogLevel)
	return c
}

// ParseAddr parses a net.TCPAddr from an IP address and port.
func ParseAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: parseIP(ip), Port: port}
}

// ParseAddrs parses an array of net.TCPAddrs from an array of IPv4:Port address strings.
func ParseAddrs(addrs []string) ([]*net.TCPAddr, error) {
	netAddrs := make([]*net.TCPAddr, len(addrs))
	for i, a := range addrs {
		parts := strings.SplitN(a, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("address not delimited by ':'")
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		netAddrs[i] = ParseAddr(parts[0], port)
	}
	return netAddrs, nil
}

// parseIP parses a string IP address and handles localhost on its own.
func parseIP(ip string) net.IP {
	if ip == "localhost" {
		return net.ParseIP("127.0.0.1")
	}
	return net.ParseIP(ip)
}

func defaultDataDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, DataSubdir)
}

func defaultDBDir(dataDir string, localName string, dbSubDir string) string {
	return filepath.Join(dataDir, localName, dbSubDir)
}

// localPeerName gives the local name (on the host) of the node using the NodeIndex
func nameFromAddr(localAddr fmt.Stringer) string {
	addrHash := md5.Sum([]byte(localAddr.String()))
	return fmt.Sprintf("peer-%x", addrHash[:4])
}
