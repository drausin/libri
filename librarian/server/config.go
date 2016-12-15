package server

import (
	"net"
	"os"
	"path/filepath"
)

var (
	defaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 11000}
	defaultDataSubdir = "data"
	defaultDbSubDir = "db"
)

// Config is used to configure a Librarian server
type Config struct {
	// DataDir is the local directory to store the state in
	DataDir       string

	// DbDir is the local directory used by the DB
	DbDir         string

	// NodeName is the name of the Librarian node
	NodeName      string

	// RPCLocalAddr is the RPC address used by the server. This should be reachable
	// by the WAN and LAN
	RPCLocalAddr  *net.TCPAddr

	// RPCPublicAddr is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:8300", but this address must be
	// reachable
	RPCPublicAddr *net.TCPAddr
}

// DefaultConfig returns a reasonable default server configuration.
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dataDir := filepath.Join(cwd, defaultDataSubdir)
	dbDir := filepath.Join(dataDir, defaultDbSubDir)

	return &Config{
		DataDir: dataDir,
		DbDir: dbDir,
		NodeName: hostname,
		RPCLocalAddr: defaultRPCAddr,
	}
}

// SetDataDir sets the data directory (and dependent directories).
func (c *Config) SetDataDir(dataDir string) {
	c.DataDir = dataDir
	c.DbDir = filepath.Join(dataDir, defaultDbSubDir)
}
