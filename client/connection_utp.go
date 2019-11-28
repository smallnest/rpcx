// +build utp

package client

import (
	"net"

	"github.com/anacrolix/utp"
)

func init() {
	ConnFactories["utp"] = newDirectUTPConn
}

func newDirectUTPConn(c *Client, network, address string) (net.Conn, error) {
	return utp.Dial(address)
}
