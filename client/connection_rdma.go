//go:build linux
// +build linux

package client

import (
	"errors"
	"net"

	"github.com/smallnest/rsocket"
)

func init() {
	ConnFactories["rdma"] = newRDMAConn
}

func newRDMAConn(c *Client, network, address string) (net.Conn, error) {
	if network != "rdma" {
		return nil, errors.New("network is not rdma")
	}

	return rsocket.DialTCP(address)
}
