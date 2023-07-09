package client

import (
	"net"
)

// experimental
func newIOUringConn(c *Client, network, address string) (net.Conn, error) {
	return newDirectConn(c, "tcp", address)
}
