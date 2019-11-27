// +build quic

package client

import (
	"crypto/tls"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
)

func newDirectQuicConn(c *Client, network, address string) (net.Conn, error) {
	tlsConf := c.option.TLSConfig
	if tlsConf == nil {
		tlsConf = &tls.Config{InsecureSkipVerify: true}
	}

	return quicconn.Dial(address, tlsConf)
}
