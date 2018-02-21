// +build utp

package client

import (
	"net"

	"github.com/anacrolix/utp"
)

func init() {
	makeConnMap["utp"] = newDirectUTPConn
}

func newDirectUTPConn(c *Client, network, address string) (net.Conn, error) {
	return utp.Dial(address)
}
