package server

import (
	"net"
	"io"
	"os"
)

var (
	DefaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 11000}
)

// Config is used to configure a Librarian server
type Config struct {
	// DataDir is the local directory to store the state in
	DataDir string

	// NodeName is the name of the Librarian node
	NodeName string

	// RPCAddr is the RPC address used by the server. This should be reachable
	// by the WAN and LAN
	RPCAddr *net.TCPAddr

	// RPCAdvertise is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:8300", but this address must be
	// reachable
	RPCAdvertise *net.TCPAddr

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer
}

// DefaultConfig returns a reasonable default server configuration.
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return &Config{
		NodeName: hostname,
		RPCAddr: DefaultRPCAddr,
	}
}
