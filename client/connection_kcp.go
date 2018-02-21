// +build kcp

package client

import (
	"net"

	kcp "github.com/xtaci/kcp-go"
)

func newDirectKCPConn(c *Client, network, address string) (net.Conn, error) {
	return kcp.DialWithOptions(address, c.option.Block.(kcp.BlockCrypt), 10, 3)
}
