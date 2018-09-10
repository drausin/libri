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
	netAddrs := make([]*net.TCPAddr, 0, len(addrs))
	nErrs := 0
	for _, a := range addrs {
		netAddr, err := net.ResolveTCPAddr("tcp4", a)
		if err != nil {
			nErrs++
			if nErrs == len(addrs) {
				// bail if none of the addrs could be parsed
				return nil, err
			}
			continue
		}
		netAddrs = append(netAddrs, netAddr)
	}
	return netAddrs, nil
}
