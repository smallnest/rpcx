// +build kcp

package client

import (
	"net"

	kcp "github.com/xtaci/kcp-go"
)

func newDirectKCPConn(c *Client, network, address string) (net.Conn, error) {
	var conn net.Conn
	var err error

	conn, err = kcp.DialWithOptions(address, c.option.Block.(kcp.BlockCrypt), 10, 3)

	if err != nil {
		return nil, err
	}

	return conn, nil
}
