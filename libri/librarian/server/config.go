package server

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (

	// DefaultPort is the default port of both local and public addresses.
	DefaultPort = 20100

	// DefaultIP is the default IP of both local and public addresses.
	DefaultIP = "localhost"

	// DefaultLogLevel is the default log level to use.
	DefaultLogLevel = zap.InfoLevel

	// DataSubdir is the name of the data directory.
	DataSubdir = "librarian-data"

	// DBSubDir is the default DB subdirectory within the data dir.
	DBSubDir = "db"
)

// Config is used to configure a Librarian server
type Config struct {
	// LocalAddr is the local address the server listens to.
	LocalAddr *net.TCPAddr

	// PublicAddr is the public address clients make requests to.
	PublicAddr *net.TCPAddr

	// PublicName is the public facing name of the peer.
	PublicName string

	// DataDir is the directory on the local machine where the state and output of all the
	// peer running on that machine are stored.
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// BootstrapAddrs is a list of addresses for bootstrap peers.
	BootstrapAddrs []*net.TCPAddr

	// Routing defines parameters for the server's routing table.
	Routing *routing.Parameters

	// Introduce defines parameters for introductions the server performs.
	Introduce *introduce.Parameters

	// Search defines parameters for searches the server performs.
	Search *search.Parameters

	// Store defines parameters for stores the server performs.
	Store *store.Parameters

	// SubscribeTo defines parameters for subscriptions to other peers.
	SubscribeTo *subscribe.ToParameters

	// SubscribeFrom defines parameters for subscriptions to other peers.
	SubscribeFrom *subscribe.FromParameters

	// LogLevel is the log level
	LogLevel zapcore.Level
}

// NewDefaultConfig returns a reasonable default server configuration.
func NewDefaultConfig() *Config {
	config := &Config{}

	// set defaults via zero values; in cases where the config B depends on config A, config A
	// should be set before config B
	config.WithDefaultLocalAddr()
	config.WithDefaultPublicAddr()
	config.WithDefaultPublicName()
	config.WithDefaultDataDir()
	config.WithDefaultDBDir()
	config.WithDefaultBootstrapAddrs()
	config.WithDefaultRouting()
	config.WithDefaultIntroduce()
	config.WithDefaultSearch()
	config.WithDefaultStore()
	config.WithDefaultSubscribeTo()
	config.WithDefaultSubscribeFrom()
	config.WithDefaultLogLevel()

	return config
}

// WithLocalAddr sets config's local address to the given value or to the default if the given
// value is nil.
func (c *Config) WithLocalAddr(localAddr *net.TCPAddr) *Config {
	if localAddr == nil {
		return c.WithDefaultLocalAddr()
	}
	c.LocalAddr = localAddr
	return c
}

// WithDefaultLocalAddr sets the local address to the default value.
func (c *Config) WithDefaultLocalAddr() *Config {
	addr, err := ParseAddr(DefaultIP, DefaultPort)
	if err != nil {
		// should never happen with default
		panic(err)
	}
	c.LocalAddr = addr
	return c
}

// WithPublicAddr sets the public address to the given value or to the default if the given value
// is nil.
func (c *Config) WithPublicAddr(publicAddr *net.TCPAddr) *Config {
	if publicAddr == nil {
		return c.WithDefaultPublicAddr()
	}
	c.PublicAddr = publicAddr
	return c
}

// WithDefaultPublicAddr sets the public address to the local address, useful when just running
// a cluster locally.
func (c *Config) WithDefaultPublicAddr() *Config {
	c.PublicAddr = c.LocalAddr
	return c
}

// WithPublicName sets the public name to the given value or the default if the given value is
// empty.
func (c *Config) WithPublicName(publicName string) *Config {
	if publicName == "" {
		return c.WithDefaultPublicName()
	}
	c.PublicName = publicName
	return c
}

// WithDefaultPublicName sets the public name to the default value, which uses a hash of the
// public address.
func (c *Config) WithDefaultPublicName() *Config {
	c.PublicName = NameFromAddr(c.PublicAddr)
	return c
}

// WithDataDir sets the data dir to the given value or the default if the given value is empty.
func (c *Config) WithDataDir(dataDir string) *Config {
	if dataDir == "" {
		return c.WithDefaultDataDir()
	}
	c.DataDir = dataDir
	return c
}

// WithDefaultDataDir sets the data dir to a 'data' subdir of the current working directory..
func (c *Config) WithDefaultDataDir() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	c.DataDir = filepath.Join(cwd, DataSubdir)
	return c
}

// WithDBDir sets the DB dir to the given value or the default if the given value is empty.
func (c *Config) WithDBDir(dbDir string) *Config {
	if dbDir == "" {
		return c.WithDefaultDBDir()
	}
	c.DbDir = dbDir
	return c
}

// WithDefaultDBDir sets the DB dir to a local name subdir of the data dir.
func (c *Config) WithDefaultDBDir() *Config {
	c.DbDir = filepath.Join(c.DataDir, DBSubDir)
	return c
}

// WithBootstrapAddrs sets the bootstrap addresses to the given value or the default if the given
// value is empty.
func (c *Config) WithBootstrapAddrs(bootstrapAddrs []*net.TCPAddr) *Config {
	if bootstrapAddrs == nil {
		return c.WithDefaultBootstrapAddrs()
	}
	c.BootstrapAddrs = bootstrapAddrs
	return c
}

// WithDefaultBootstrapAddrs sets the bootstrap addresses to a single address of the default IP
// and port.
func (c *Config) WithDefaultBootstrapAddrs() *Config {
	// default is itself
	addr, err := ParseAddr(DefaultIP, DefaultPort)
	if err != nil {
		// should never happen with default
		panic(err)
	}
	c.BootstrapAddrs = []*net.TCPAddr{addr}
	return c
}

// WithRouting sets the routing parameters to the given value or the default if it is nil.
func (c *Config) WithRouting(params *routing.Parameters) *Config {
	if params == nil {
		return c.WithDefaultRouting()
	}
	c.Routing = params
	return c
}

// WithDefaultRouting sets the routing parameters to the default values specified in the routing
// module.
func (c *Config) WithDefaultRouting() *Config {
	c.Routing = routing.NewDefaultParameters()
	return c
}

// WithIntroduce sets the introduce parameters to the given value or the default if it is nil.
func (c *Config) WithIntroduce(params *introduce.Parameters) *Config {
	if params == nil {
		return c.WithDefaultIntroduce()
	}
	c.Introduce = params
	return c
}

// WithDefaultIntroduce sets the introduce parameters to the default values specified in the
// introduce package.
func (c *Config) WithDefaultIntroduce() *Config {
	c.Introduce = introduce.NewDefaultParameters()
	return c
}

// WithSearch sets the search parameters to the given value or the default if it is nil.
func (c *Config) WithSearch(params *search.Parameters) *Config {
	if params == nil {
		return c.WithDefaultSearch()
	}
	c.Search = params
	return c
}

// WithDefaultSearch sets the search parameters to their default values specified in the search
// package.
func (c *Config) WithDefaultSearch() *Config {
	c.Search = search.NewDefaultParameters()
	return c
}

// WithStore sets the store parameters to the given value or the default if it is nil.
func (c *Config) WithStore(params *store.Parameters) *Config {
	if params == nil {
		return c.WithDefaultStore()
	}
	c.Store = params
	return c
}

// WithDefaultStore sets the store parameters to their default values specified in the store
// package.
func (c *Config) WithDefaultStore() *Config {
	c.Store = store.NewDefaultParameters()
	return c
}

// WithSubscribeTo sets the subscription to parameters to the given value or the default it it is
// nil.
func (c *Config) WithSubscribeTo(params *subscribe.ToParameters) *Config {
	if params == nil {
		return c.WithDefaultSubscribeTo()
	}
	c.SubscribeTo = params
	return c
}

// WithDefaultSubscribeTo sets the subscription to parameters to the default.
func (c *Config) WithDefaultSubscribeTo() *Config {
	c.SubscribeTo = subscribe.NewDefaultToParameters()
	return c
}

// WithSubscribeFrom sets the subscription from parameters to the given value or the default it is
// nil.
func (c *Config) WithSubscribeFrom(params *subscribe.FromParameters) *Config {
	if params == nil {
		return c.WithDefaultSubscribeFrom()
	}
	c.SubscribeFrom = params
	return c
}

// WithDefaultSubscribeFrom sets the subscription from parameters to the default.
func (c *Config) WithDefaultSubscribeFrom() *Config {
	c.SubscribeFrom = subscribe.NewDefaultFromParameters()
	return c
}

// WithLogLevel sets the log level to the given value, though this doesn't have any direct effect
// on the creation of the logger instance.
func (c *Config) WithLogLevel(logLevel zapcore.Level) *Config {
	if logLevel == 0 {
		return c.WithDefaultLogLevel()
	}
	c.LogLevel = logLevel
	return c
}

// WithDefaultLogLevel sets the log level to INFO.
func (c *Config) WithDefaultLogLevel() *Config {
	c.LogLevel = DefaultLogLevel
	return c
}

func (c *Config) isBootstrap() bool {
	for _, a := range c.BootstrapAddrs {
		if c.PublicAddr.String() == a.String() {
			return true
		}
	}
	return false
}

// ParseAddr parses a net.TCPAddr from a host address and port.
func ParseAddr(host string, port int) (*net.TCPAddr, error) {
	return net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
}

// ParseAddrs parses an array of net.TCPAddrs from an array of IPv4:Port address strings.
func ParseAddrs(addrs []string) ([]*net.TCPAddr, error) {
	netAddrs := make([]*net.TCPAddr, len(addrs))
	for i, a := range addrs {
		netAddr, err := net.ResolveTCPAddr("tcp4", a)
		netAddrs[i] = netAddr
		if err != nil {
			return nil, err
		}
	}
	return netAddrs, nil
}

// NameFromAddr gives the local name (on the host) of the node using the NodeIndex
func NameFromAddr(localAddr fmt.Stringer) string {
	addrHash := sha256.Sum256([]byte(localAddr.String()))
	return fmt.Sprintf("peer-%x", addrHash[:4])
}
