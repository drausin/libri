package parse

import (
	"fmt"
	"net"
)

// Addr parses a net.TCPAddr from a host address and port.
func Addr(host string, port int) (*net.TCPAddr, error) {
	return net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
}

// Addrs parses an array of net.TCPAddrs from an array of IPv4:Port address strings.
func Addrs(addrs []string) ([]*net.TCPAddr, error) {
	netAddrs := make([]*net.TCPAddr, len(addrs))
	for i, a := range addrs {
		netAddr, err := net.ResolveTCPAddr("tcp4", a)
		if err != nil {
			return nil, err
		}
		netAddrs[i] = netAddr
	}
	return netAddrs, nil
}
