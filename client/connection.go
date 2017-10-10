package client

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	quicconn "github.com/marten-seemann/quic-conn"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/share"
	kcp "github.com/xtaci/kcp-go"
)

// Connect connects the server via specified network.
func (c *Client) Connect(network, address string, opts ...interface{}) error {
	var conn net.Conn
	var err error

	switch network {
	case "http":
		conn, err = newDirectHTTPConn(c, network, address)
	case "kcp":
		conn, err = newDirectKCPConn(c, network, address)
	case "quic":
		conn, err = newDirectQuicConn(c, network, address)
	default:
		conn, err = newDirectTCPConn(c, network, address)
	}

	if err == nil && conn != nil {
		if c.ReadTimeout != 0 {
			conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
		}
		if c.WriteTimeout != 0 {
			conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
		}

		c.Conn = conn
		c.r = bufio.NewReaderSize(conn, ReaderBuffsize)
		c.w = bufio.NewWriterSize(conn, WriterBuffsize)

		// start reading and writing since connected
		go c.input()
	}

	return err
}

func newDirectTCPConn(c *Client, network, address string, opts ...interface{}) (net.Conn, error) {
	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c != nil && c.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: c.ConnectTimeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, network, address, c.TLSConfig)
		//or conn:= tls.Client(netConn, &config)
		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout(network, address, c.ConnectTimeout)
	}

	if err != nil {
		log.Errorf("failed to dial server: %v", err)
		return nil, err
	}

	return conn, nil
}

var connected = "200 Connected to rpcx"

func newDirectHTTPConn(c *Client, network, address string, opts ...interface{}) (net.Conn, error) {
	var path string

	if len(opts) == 0 {
		path = opts[0].(string)
	} else {
		path = share.DefaultRPCPath
	}

	network = "tcp"

	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c != nil && c.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: c.ConnectTimeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, network, address, c.TLSConfig)
		//or conn:= tls.Client(netConn, &config)

		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout(network, address, c.ConnectTimeout)
	}
	if err != nil {
		log.Errorf("failed to dial server: %v", err)
		return nil, err
	}

	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return conn, nil
	}
	if err == nil {
		log.Errorf("unexpected HTTP response: %v", err)
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  network + " " + address,
		Addr: nil,
		Err:  err,
	}
}

func newDirectKCPConn(c *Client, network, address string, opts ...interface{}) (net.Conn, error) {
	var conn net.Conn
	var err error

	conn, err = kcp.DialWithOptions(address, c.Block, 10, 3)

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
