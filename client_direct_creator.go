package rpcx

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"time"

	kcp "github.com/xtaci/kcp-go"
)

// NewDirectRPCClient creates a rpc client
func NewDirectRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, timeout time.Duration) (*rpc.Client, error) {
	//if network == "http" || network == "https" {
	switch network {
	case "http":
		return NewDirectHTTPRPCClient(c, clientCodecFunc, network, address, "", timeout)
	case "kcp":
		return NewDirectKCPRPCClient(c, clientCodecFunc, network, address, "", timeout)
	default:
	}

	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c != nil && c.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: timeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, network, address, c.TLSConfig)
		//or conn:= tls.Client(netConn, &config)

		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout(network, address, timeout)
	}

	if err != nil {
		return nil, err
	}

	return wrapConn(c, clientCodecFunc, conn)
}

func wrapConn(c *Client, clientCodecFunc ClientCodecFunc, conn net.Conn) (*rpc.Client, error) {
	var ok bool
	if conn, ok = c.PluginContainer.DoPostConnected(conn); !ok {
		return nil, errors.New("failed to do post connected")
	}

	if c == nil || c.PluginContainer == nil {
		return rpc.NewClientWithCodec(clientCodecFunc(conn)), nil
	}

	wrapper := newClientCodecWrapper(c.PluginContainer, clientCodecFunc(conn), conn)
	wrapper.Timeout = c.Timeout
	wrapper.ReadTimeout = c.ReadTimeout
	wrapper.WriteTimeout = c.WriteTimeout

	return rpc.NewClientWithCodec(wrapper), nil
}

// NewDirectHTTPRPCClient creates a rpc http client
func NewDirectHTTPRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, path string, timeout time.Duration) (*rpc.Client, error) {
	if path == "" {
		path = rpc.DefaultRPCPath
	}

	var conn net.Conn
	var tlsConn *tls.Conn
	var err error

	if c != nil && c.TLSConfig != nil {
		dialer := &net.Dialer{
			Timeout: timeout,
		}
		tlsConn, err = tls.DialWithDialer(dialer, "tcp", address, c.TLSConfig)
		//or conn:= tls.Client(netConn, &config)

		conn = net.Conn(tlsConn)
	} else {
		conn, err = net.DialTimeout("tcp", address, timeout)
	}
	if err != nil {
		return nil, err
	}

	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		if c == nil || c.PluginContainer == nil {
			return rpc.NewClientWithCodec(clientCodecFunc(conn)), nil
		}
		wrapper := newClientCodecWrapper(c.PluginContainer, clientCodecFunc(conn), conn)
		wrapper.Timeout = c.Timeout
		wrapper.ReadTimeout = c.ReadTimeout
		wrapper.WriteTimeout = c.WriteTimeout

		return rpc.NewClientWithCodec(wrapper), nil
	}
	if err == nil {
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

// NewDirectKCPRPCClient creates a kcp client.
// kcp project: https://github.com/xtaci/kcp-go
func NewDirectKCPRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, path string, timeout time.Duration) (*rpc.Client, error) {
	var conn net.Conn
	var err error

	conn, err = kcp.DialWithOptions(address, c.Block, 10, 3)

	if err != nil {
		return nil, err
	}

	return wrapConn(c, clientCodecFunc, conn)
}
