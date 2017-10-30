// +build !quic

package client

import (
	"errors"
	"net"
)

func newDirectQuicConn(c *Client, network, address string) (net.Conn, error) {
	return nil, errors.New("quic unsupported")
}
