package rpcx

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
)

// SelectMode defines the algorithm of selecting a services from cluster
type SelectMode int

const (
	RandomSelect SelectMode = iota
	RoundRobin
	LeastActive
	ConsistentHash
)

var selectModeStrs = [...]string{
	"RandomSelect",
	"RoundRobin",
	"LeastActive",
	"ConsistentHash",
}

func (s SelectMode) String() string {
	return selectModeStrs[s]
}

//FailMode is a feature to decide client actions when clients fail to invoke services
type FailMode int

const (
	//Failover selects another server automaticaly
	Failover FailMode = iota
	//Failfast returns error immediately
	Failfast
	//Failtry use current client again
	Failtry
)

//ClientSelector defines an interface to create a rpc.Client from cluster or standalone.
type ClientSelector interface {
	Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*rpc.Client, error)
	SetClient(*Client)
}

// DirectClientSelector is used to a direct rpc server.
// It don't select a node from service cluster but a specific rpc server.
type DirectClientSelector struct {
	Network, Address string
	Timeout          time.Duration
	Client           *Client
}

//Select returns a rpc client
func (s *DirectClientSelector) Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	return NewDirectRPCClient(s.Client, clientCodecFunc, s.Network, s.Address, s.Timeout)
}

func (s *DirectClientSelector) SetClient(c *Client) {
	s.Client = c
}

// NewDirectRPCClient creates a rpc client
func NewDirectRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, timeout time.Duration) (*rpc.Client, error) {
	//if network == "http" || network == "https" {
	if network == "http" {
		return NewDirectHTTPRPCClient(c, clientCodecFunc, network, address, "", timeout)
	}
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}

	if c == nil || c.PluginContainer == nil {
		return rpc.NewClientWithCodec(clientCodecFunc(conn)), nil
	}
	return rpc.NewClientWithCodec(newClientCodecWrapper(c.PluginContainer, clientCodecFunc(conn))), nil
}

// NewDirectHTTPRPCClient creates a rpc http client
func NewDirectHTTPRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, path string, timeout time.Duration) (*rpc.Client, error) {
	if path == "" {
		path = rpc.DefaultRPCPath
	}

	var err error
	conn, err := net.DialTimeout("tcp", address, timeout)
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
		return rpc.NewClientWithCodec(newClientCodecWrapper(c.PluginContainer, clientCodecFunc(conn))), nil
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

// ClientCodecFunc is used to create a rpc.ClientCodecFunc from net.Conn.
type ClientCodecFunc func(conn io.ReadWriteCloser) rpc.ClientCodec

// Client represents a RPC client.
type Client struct {
	rpcClient       *rpc.Client
	ClientSelector  ClientSelector
	ClientCodecFunc ClientCodecFunc
	PluginContainer IClientPluginContainer
	FailMode        FailMode
	Retries         int
}

//NewClient create a client.
func NewClient(s ClientSelector) *Client {
	client := &Client{
		PluginContainer: &ClientPluginContainer{plugins: make([]IPlugin, 0)},
		ClientCodecFunc: msgpackrpc.NewClientCodec,
		ClientSelector:  s,
		FailMode:        Failfast,
		Retries:         3}
	s.SetClient(client)
	return client
}

// Close closes the connection
func (c *Client) Close() error {
	if c.rpcClient != nil {
		return c.rpcClient.Close()
	}
	return nil
}

//Call invokes the named function, waits for it to complete, and returns its error status.
func (c *Client) Call(serviceMethod string, args interface{}, reply interface{}) (err error) {
	var rpcClient *rpc.Client
	if c.rpcClient == nil {
		rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
		c.rpcClient = rpcClient

	}
	if err == nil && c.rpcClient != nil {
		err = c.rpcClient.Call(serviceMethod, args, reply)
	}
	if err != nil || c.rpcClient == nil {
		if c.FailMode == Failover {
			for retries := 0; retries < c.Retries; retries++ {
				rpcClient, err := c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
				if err != nil || rpcClient == nil {
					continue
				}
				c.Close()

				c.rpcClient = rpcClient
				err = c.rpcClient.Call(serviceMethod, args, reply)
				if err == nil {
					return nil
				}
			}
		} else if c.FailMode == Failtry {
			for retries := 0; retries < c.Retries; retries++ {
				if c.rpcClient == nil {
					rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
					c.rpcClient = rpcClient

				}
				if c.rpcClient != nil {
					err = c.rpcClient.Call(serviceMethod, args, reply)
					if err == nil {
						return nil
					}
				}
			}
		}
	}

	return
}

//Go invokes the function asynchronously. It returns the Call structure representing the invocation.
//The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
func (c *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	if c.rpcClient == nil {
		rpcClient, _ := c.ClientSelector.Select(c.ClientCodecFunc)
		c.rpcClient = rpcClient

	}
	return c.rpcClient.Go(serviceMethod, args, reply, done)
}

// Auth sets Authorization info
func (c *Client) Auth(authorization, tag string) error {
	p := NewAuthorizationClientPlugin(authorization, tag)
	return c.PluginContainer.Add(p)
}

type clientCodecWrapper struct {
	rpc.ClientCodec
	PluginContainer IClientPluginContainer
}

// newClientCodecWrapper wraps a rpc.ServerCodec.
func newClientCodecWrapper(pc IClientPluginContainer, c rpc.ClientCodec) *clientCodecWrapper {
	return &clientCodecWrapper{ClientCodec: c, PluginContainer: pc}
}

func (w *clientCodecWrapper) ReadRequestHeader(r *rpc.Response) error {
	//pre
	err := w.PluginContainer.DoPreReadResponseHeader(r)
	if err != nil {
		return err
	}

	err = w.ClientCodec.ReadResponseHeader(r)
	if err != nil {
		return err
	}

	//post
	return w.PluginContainer.DoPostReadResponseHeader(r)
}

func (w *clientCodecWrapper) ReadRequestBody(body interface{}) error {
	//pre
	err := w.PluginContainer.DoPreReadResponseBody(body)
	if err != nil {
		return err
	}

	err = w.ClientCodec.ReadResponseBody(body)
	if err != nil {
		return err
	}

	//post
	return w.PluginContainer.DoPostReadResponseBody(body)
}

func (w *clientCodecWrapper) WriteRequest(r *rpc.Request, body interface{}) error {
	//pre
	err := w.PluginContainer.DoPreWriteRequest(r, body)
	if err != nil {
		return err
	}

	err = w.ClientCodec.WriteRequest(r, body)
	if err != nil {
		return err
	}

	//post
	return w.PluginContainer.DoPostWriteRequest(r, body)
}

func (w *clientCodecWrapper) Close() error {
	return w.ClientCodec.Close()
}
