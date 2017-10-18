// +build udp

package client

import (
	"crypto/tls"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
	kcp "github.com/xtaci/kcp-go"
)

func newDirectKCPConn(c *Client, network, address string, opts ...interface{}) (net.Conn, error) {
	var conn net.Conn
	var err error

	conn, err = kcp.DialWithOptions(address, c.Block.(kcp.BlockCrypt), 10, 3)

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func newDirectQuicConn(c *Client, network, address string, opts ...interface{}) (net.Conn, error) {
	var conn net.Conn
	var err error

	tlsConf := &tls.Config{InsecureSkipVerify: true}
	conn, err = quicconn.Dial(address, tlsConf)

	if err != nil {
		return nil, err
	}

	return conn, nil
}
