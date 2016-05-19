package betterrpc

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

//ClientSelector defines an interface to create a rpc.Client from cluster or standalone.
type ClientSelector interface {
	Select(clientCodecFunc ClientCodecFunc) (*rpc.Client, error)
}

// DirectClientSelector is used to a direct rpc server.
// It don't select a node from service cluster but a specific rpc server.
type DirectClientSelector struct {
	Network, Address string
	timeout          time.Duration
}

//Select returns a rpc client
func (s *DirectClientSelector) Select(clientCodecFunc ClientCodecFunc) (*rpc.Client, error) {
	return NewDirectRPCClient(clientCodecFunc, s.Network, s.Address, s.timeout)
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
}

//NewClient create a client.
func NewClient(s ClientSelector) *Client {
	return &Client{
		PluginContainer: &ClientPluginContainer{plugins: make([]IPlugin, 0)},
		ClientCodecFunc: msgpackrpc.NewClientCodec,
		ClientSelector:  s}
}

// Start starts a rpc clent.
func (c *Client) Start() error {
	rpcClient, err := c.ClientSelector.Select(c.ClientCodecFunc)
	c.rpcClient = rpcClient
	return err
}

// Close closes the connection
func (c *Client) Close() error {
	return c.rpcClient.Close()
}

//Call invokes the named function, waits for it to complete, and returns its error status.
func (c *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	return c.rpcClient.Call(serviceMethod, args, reply)
}

//Go invokes the function asynchronously. It returns the Call structure representing the invocation.
//The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
func (c *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
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
