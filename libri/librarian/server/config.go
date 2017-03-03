package server

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

const (
	DefaultPort       = 11000
	DefaultIP         = "localhost"
	DefaultDataSubdir = "data"
	DefaultDbSubDir   = "db"
)

// Config is used to configure a Librarian server
type Config struct {
	// PeerName is the public facing name of the peer.
	PeerName string

	// LocalName is the peer name on the particular box.
	LocalName string

	// DataDir is the local directory to store the node states
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// LocalAddr is the local address the server listens to.
	LocalAddr *net.TCPAddr

	// PublicAddr is the public address clients make requests to.
	PublicAddr *net.TCPAddr

	// BootstrapAddrs
	BootstrapAddrs []*net.TCPAddr
}

// DefaultConfig returns a reasonable default server configuration.
func DefaultConfig() *Config {
	ln := localPeerName(DefaultIP, DefaultPort)
	nn, err := nodeName(ln)
	if err != nil {
		panic(err)
	}
	ddir, err := dataDir()
	if err != nil {
		panic(err)
	}
	dbdir := dbDir(ddir, ln)

	return &Config{
		LocalName:  ln,
		PeerName:   nn,
		DataDir:    ddir,
		DbDir:      dbdir,
		LocalAddr:    &net.TCPAddr{IP: ParseIP(DefaultIP), Port: DefaultPort},
		PublicAddr:    &net.TCPAddr{IP: ParseIP(DefaultIP), Port: DefaultPort},
		BootstrapAddrs: make([]*net.TCPAddr, 0),
	}
}

// SetDataDir sets the config's data directory, which also sets the database directory.
func (c *Config) WithDataDir(dataDir string) {
	c.DataDir = dataDir
	c.DbDir = dbDir(dataDir, c.LocalName)
}

// ParseIP parses a string IP address and handles localhost on its own.
func ParseIP(ip string) net.IP {
	if ip == "localhost" {
		return net.ParseIP("127.0.0.1")
	}
	return net.ParseIP(ip)
}

func dataDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, DefaultDataSubdir), nil
}

// dbDir gets the database directory from the main data directory.
func dbDir(dataDir, localNodeName string) string {
	return filepath.Join(dataDir, localNodeName, DefaultDbSubDir)
}

// nodeName gives the node name from the hostname and local node name.
func nodeName(localName string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", hostname, localName), nil
}

// localPeerName gives the local name (on the host) of the node using the NodeIndex
func localPeerName(ip string, port int) string {
	addrHash := md5.Sum([]byte(fmt.Sprintf("%s:%d", ip, port)))
	return fmt.Sprintf("peer-%x", addrHash[:4])
}

