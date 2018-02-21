// +build rudp

package client

import (
	"net"
)

func init() {
	makeConnMap["rudp"] = newDirectRUDPConn
}

func newDirectRUDPConn(c *Client, network, address string) (net.Conn, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	laddr := net.UDPAddr{IP: net.IPv4zero, Port: 0}
	return net.DialUDP("udp", &laddr, addr)
}
