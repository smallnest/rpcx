// +build !udp

package client

import (
	"errors"
	"net"
)

func newDirectKCPConn(c *Client, network, address string) (net.Conn, error) {
	return nil, errors.New("kcp unsupported")
}

func newDirectQuicConn(c *Client, network, address string) (net.Conn, error) {
	return nil, errors.New("quic unsupported")
}
