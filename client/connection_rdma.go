//go:build rdma
// +build rdma

package client

import (
	"errors"
	"net"

	"github.com/smallnest/gordma/rdmanet"
	"github.com/smallnest/rpcx/share"
)

func init() {
	ConnFactories["rdma"] = newRDMAConn
}

func newRDMAConn(c *Client, network, address string) (net.Conn, error) {
	if network != "rdma" {
		return nil, errors.New("network is not rdma")
	}

	conn, err := rdmanet.DialTimeout(address, c.option.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	return share.NewRDMAConn(conn), nil
}
