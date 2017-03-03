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
)

const (
	DefaultPort       = 11000
	DefaultIP         = "localhost"
	DefaultDataSubdir = "data"
	DefaultDbSubDir   = "db"
	DefaultLogLevel   = zap.InfoLevel
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

	// DataDir is the local directory to store the node states
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
	config.WithLocalAddr(nil)
	config.WithPublicAddr(nil)
	config.WithLocalName("")
	config.WithPublicName("")
	config.WithDataDir("")
	config.WithDBDir("")
	config.WithBootstrapAddrs(nil)

	return config
}

func (c *Config) WithLocalAddr(localAddr *net.TCPAddr) *Config {
	if localAddr == nil {
		localAddr = &net.TCPAddr{IP: parseIP(DefaultIP), Port: DefaultPort}
	}
	c.LocalAddr = localAddr
	return c
}

func (c *Config) WithPublicAddr(publicAddr *net.TCPAddr) *Config {
	if publicAddr == nil {
		publicAddr = c.LocalAddr
	}
	c.PublicAddr = publicAddr
	return c
}

func (c *Config) WithLocalName(localName string) *Config {
	if localName != "" {
		localName = nameFromAddr(c.LocalAddr)
	}
	c.LocalName = localName
	return c
}

func (c *Config) WithPublicName(publicName string) *Config {
	if publicName == "" {
		publicName = nameFromAddr(c.PublicAddr)
	}
	c.PublicName = publicName
	return c
}

func (c *Config) WithDataDir(dataDir string) *Config {
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	c.DataDir = dataDir
	return c
}

func (c *Config) WithDBDir(dbDir string) *Config {
	if dbDir == "" {
		dbDir = defaultDBDir(c.DataDir, c.LocalName, DefaultDbSubDir)
	}
	c.DbDir = dbDir
	return c
}

func (c *Config) WithBootstrapAddrs(bootstrapAddrs []*net.TCPAddr) *Config {
	if bootstrapAddrs == nil {
		bootstrapAddrs = make([]*net.TCPAddr, 0)
	}
	c.BootstrapAddrs = bootstrapAddrs
	return c
}

func (c *Config) WithLogLevel(logLevel zapcore.Level) *Config {
	if logLevel == 0 {
		logLevel = DefaultLogLevel
	}
	c.LogLevel = logLevel
	return c
}

func ParseAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: parseIP(ip), Port: port}
}

func ParseAddrs(addrs []string) ([]*net.TCPAddr, error) {
	netAddrs := make([]*net.TCPAddr, len(addrs))
	for i, a := range addrs {
		parts := strings.SplitN(a, ":", 2)
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
	return filepath.Join(cwd, DefaultDataSubdir)
}

func defaultDBDir(dataDir string, localName string, dbSubDir string) string {
	return filepath.Join(dataDir, localName, dbSubDir)
}

// localPeerName gives the local name (on the host) of the node using the NodeIndex
func nameFromAddr(localAddr *net.TCPAddr) string {
	addrHash := md5.Sum([]byte(localAddr.String()))
	return fmt.Sprintf("peer-%x", addrHash[:4])
}
