package client

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/smallnest/rpcx/v5/log"
	"github.com/smallnest/rpcx/v5/share"
)

type ConnFactoryFn func(c *Client, network, address string) (net.Conn, error)

var ConnFactories = map[string]ConnFactoryFn{
	"http": newDirectHTTPConn,
	"kcp":  newDirectKCPConn,
	"quic": newDirectQuicConn,
	"unix": newDirectConn,
}

// Connect connects the server via specified network.
func (c *Client) Connect(network, address string) error {
	var conn net.Conn
	var err error

	switch network {
	case "http":
		conn, err = newDirectHTTPConn(c, network, address)
	case "kcp":
		conn, err = newDirectKCPConn(c, network, address)
	case "quic":
		conn, err = newDirectQuicConn(c, network, address)
	case "unix":
		conn, err = newDirectConn(c, network, address)
	default:
		fn := ConnFactories[network]
		if fn != nil {
			conn, err = fn(c, network, address)
		} else {
			conn, err = newDirectConn(c, network, address)
		}
	}

	if err == nil && conn != nil {
		if c.option.ReadTimeout != 0 {
			conn.SetReadDeadline(time.Now().Add(c.option.ReadTimeout))
		}
		if c.option.WriteTimeout != 0 {
			conn.SetWriteDeadline(time.Now().Add(c.option.WriteTimeout))
		}

		if c.Plugins != nil {
			conn, err = c.Plugins.DoConnCreated(conn)
			if err != nil {
				return err
			}
		}

		c.Conn = conn
		c.r = bufio.NewReaderSize(conn, ReaderBuffsize)
		//c.w = bufio.NewWriterSize(conn, WriterBuffsize)

		// start reading and writing since connected
		go c.input()

		if c.option.Heartbeat && c.option.HeartbeatInterval > 0 {
			go c.heartbeat()
		}

	}

	return err
}

func newDirectConn(c *Client, network, address string) (net.Conn, error) {
	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c != nil && c.option.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: c.option.ConnectTimeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, network, address, c.option.TLSConfig)
		//or conn:= tls.Client(netConn, &config)
		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout(network, address, c.option.ConnectTimeout)
	}

	if err != nil {
		log.Warnf("failed to dial server: %v", err)
		return nil, err
	}

	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)
	}

	return conn, nil
}

var connected = "200 Connected to rpcx"

func newDirectHTTPConn(c *Client, network, address string) (net.Conn, error) {
	if c == nil {
		return nil, errors.New("empty client")
	}
	path := c.option.RPCPath
	if path == "" {
		path = share.DefaultRPCPath
	}

	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c.option.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: c.option.ConnectTimeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, "tcp", address, c.option.TLSConfig)
		//or conn:= tls.Client(netConn, &config)

		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout("tcp", address, c.option.ConnectTimeout)
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
