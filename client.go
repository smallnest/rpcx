package rpcx

import (
	"io"
	"net"
	"net/rpc"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
)

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
}

// DirectClientSelector is used to a direct rpc server.
// It don't select a node from service cluster but a specific rpc server.
type DirectClientSelector struct {
	Network, Address string
	Timeout          time.Duration
}

//Select returns a rpc client
func (s *DirectClientSelector) Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	return NewDirectRPCClient(clientCodecFunc, s.Network, s.Address, s.Timeout)
}

// NewDirectRPCClient creates a rpc client
func NewDirectRPCClient(clientCodecFunc ClientCodecFunc, network, address string, timeout time.Duration) (*rpc.Client, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return rpc.NewClientWithCodec(clientCodecFunc(conn)), nil
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
	return &Client{
		PluginContainer: &ClientPluginContainer{plugins: make([]IPlugin, 0)},
		ClientCodecFunc: msgpackrpc.NewClientCodec,
		ClientSelector:  s,
		FailMode:        Failfast,
		Retries:         3}
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

type clientCodecWrapper struct {
	rpc.ClientCodec
	PluginContainer IClientPluginContainer
}

func (w *clientCodecWrapper) ReadRequestHeader(r *rpc.Response) error {
	//pre

	return w.ClientCodec.ReadResponseHeader(r)

	//post
}

func (w *clientCodecWrapper) ReadRequestBody(body interface{}) error {
	//pre

	return w.ClientCodec.ReadResponseBody(body)

	//post
}

func (w *clientCodecWrapper) WriteRequest(r *rpc.Request, body interface{}) error {
	//pre

	return w.ClientCodec.WriteRequest(r, body)

	//post
}

func (w *clientCodecWrapper) Close() error {
	return w.ClientCodec.Close()
}
