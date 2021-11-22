package client

import (
	"net"

	"github.com/akutz/memconn"
)

func newMemuConn(c *Client, network, address string) (net.Conn, error) {
	return memconn.Dial(network, address)
}
