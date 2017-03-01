package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

var (
	defaultRPCIP = net.ParseIP("0.0.0.0")  // localhost
	defaultRPCPort = 11000
	defaultDataSubdir = "data"
	defaultDbSubDir   = "db"
)

// Config is used to configure a Librarian server
type Config struct {
	// NodeNumber is the index (starting at 0) of the node on the host
	NodeIndex uint8

	// PeerName is the public facing name of the node.
	PeerName string

	// DataDir is the local directory to store the node states
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// RPCLocalAddr is the RPC address used by the server. This should be reachable
	// by the WAN and LAN
	RPCLocalAddr *net.TCPAddr

	// RPCPublicAddr is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:8300", but this address must be
	// reachable
	RPCPublicAddr *net.TCPAddr

	// BootstrapAddrs is a list of peer addresses to initially introduce oneself to.
	BootstrapAddrs []*net.TCPAddr
}

// DefaultConfig returns a reasonable default server configuration.
func DefaultConfig(nodeIndex uint8) *Config {
	lnn := localNodeName(nodeIndex)
	nn, err := nodeName(lnn)
	if err != nil {
		panic(err)
	}
	ddir, err := dataDir()
	if err != nil {
		panic(err)
	}
	dbdir := dbDir(ddir, lnn)

	rpcAddr := &net.TCPAddr{
		IP: defaultRPCIP,
		Port: defaultRPCPort + int(nodeIndex),
	}
	return &Config{
		NodeIndex:     nodeIndex,
		PeerName:      nn,
		DataDir:       ddir,
		DbDir:         dbdir,
		RPCPublicAddr: rpcAddr,
		RPCLocalAddr: rpcAddr,
	}
}

// SetDataDir sets the config's data directory, which also sets the database directory.
func (c *Config) SetDataDir(dataDir string) {
	c.DataDir = dataDir
	c.DbDir = dbDir(dataDir, localNodeName(c.NodeIndex))
}

func dataDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, defaultDataSubdir), nil
}

// dbDir gets the database directory from the main data directory.
func dbDir(dataDir, localNodeName string) string {
	return filepath.Join(dataDir, localNodeName, defaultDbSubDir)
}

// nodeName gives the node name from the hostname and local node name.
func nodeName(localNodeName string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", hostname, localNodeName), nil
}

// localNodeName gives the local name (on the host) of the node using the NodeIndex
func localNodeName(nodeIndex uint8) string {
	return fmt.Sprintf("node%03d", nodeIndex)
}
