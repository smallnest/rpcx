// +build quic

package client

import (
	"crypto/tls"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
)

func newDirectQuicConn(c *Client, network, address string) (net.Conn, error) {
	var conn net.Conn
	var err error

	tlsConf := c.option.TLSConfig
	if tlsConf == nil {
		tlsConf = &tls.Config{InsecureSkipVerify: true}
	}

	conn, err = quicconn.Dial(address, tlsConf)

	if err != nil {
		return nil, err
	}

	return conn, nil
}
