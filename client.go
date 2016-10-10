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

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
)

// SelectMode defines the algorithm of selecting a services from cluster
type SelectMode int

const (
	//RandomSelect is selecting randomly
	RandomSelect SelectMode = iota
	//RoundRobin is selecting by round robin
	RoundRobin
	//WeightedRoundRobin is selecting by weighted round robin
	WeightedRoundRobin
	//WeightedICMP is selecting by weighted Ping time
	WeightedICMP
	//ConsistentHash is selecting by hashing
	ConsistentHash
	//Closest is selecting the closest server
	Closest
)

var selectModeStrs = [...]string{
	"RandomSelect",
	"RoundRobin",
	"WeightedRoundRobin",
	"WeightedICMP",
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
	//Broadcast sends requests to all servers and Success only when all servers return OK
	Broadcast
	//Forking sends requests to all servers and Success once one server returns OK
	Forking
)

//ClientSelector defines an interface to create a rpc.Client from cluster or standalone.
type ClientSelector interface {
	//Select returns a new client and it also update current client
	Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*rpc.Client, error)
	//SetClient set current client
	SetClient(*Client)
	SetSelectMode(SelectMode)
	//AllClients returns all Clients
	AllClients(clientCodecFunc ClientCodecFunc) []*rpc.Client
}

// DirectClientSelector is used to a direct rpc server.
// It don't select a node from service cluster but a specific rpc server.
type DirectClientSelector struct {
	Network, Address string
	DialTimeout      time.Duration
	Client           *Client
	rpcClient        *rpc.Client
}

//Select returns a rpc client.
func (s *DirectClientSelector) Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.rpcClient != nil {
		return s.rpcClient, nil
	}
	c, err := NewDirectRPCClient(s.Client, clientCodecFunc, s.Network, s.Address, s.DialTimeout)
	s.rpcClient = c
	return c, err
}

//SetClient sets the unique client.
func (s *DirectClientSelector) SetClient(c *Client) {
	s.Client = c
}

//SetSelectMode is meaningless for DirectClientSelector because there is only one client.
func (s *DirectClientSelector) SetSelectMode(sm SelectMode) {

}

//AllClients returns rpc.Clients to all servers
func (s *DirectClientSelector) AllClients(clientCodecFunc ClientCodecFunc) []*rpc.Client {
	if s.rpcClient == nil {
		return []*rpc.Client{}
	}
	return []*rpc.Client{s.rpcClient}
}

// NewDirectRPCClient creates a rpc client
func NewDirectRPCClient(c *Client, clientCodecFunc ClientCodecFunc, network, address string, timeout time.Duration) (*rpc.Client, error) {
	//if network == "http" || network == "https" {
	if network == "http" {
		return NewDirectHTTPRPCClient(c, clientCodecFunc, network, address, "", timeout)
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

// ClientCodecFunc is used to create a rpc.ClientCodecFunc from net.Conn.
type ClientCodecFunc func(conn io.ReadWriteCloser) rpc.ClientCodec

// Client represents a RPC client.
type Client struct {
	ClientSelector  ClientSelector
	ClientCodecFunc ClientCodecFunc
	PluginContainer IClientPluginContainer
	FailMode        FailMode
	TLSConfig       *tls.Config
	Retries         int
	//Timeout sets deadline for underlying net.Conns
	Timeout time.Duration
	//Timeout sets readdeadline for underlying net.Conns
	ReadTimeout time.Duration
	//Timeout sets writedeadline for underlying net.Conns
	WriteTimeout time.Duration
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
	clients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if clients != nil {
		for _, rpcClient := range clients {
			rpcClient.Close()
		}
	}

	return nil
}

//Call invokes the named function, waits for it to complete, and returns its error status.
func (c *Client) Call(serviceMethod string, args interface{}, reply interface{}) (err error) {
	if c.FailMode == Broadcast {
		return c.clientBroadCast(serviceMethod, args, reply)
	}
	if c.FailMode == Forking {
		return c.clientForking(serviceMethod, args, reply)
	}

	//select a rpc.Client and call
	rpcClient, err := c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
	//selected
	if err == nil && rpcClient != nil {
		if err = rpcClient.Call(serviceMethod, args, reply); err == nil {
			return //call successful
		}
	}

	if c.FailMode == Failover {
		for retries := 0; retries < c.Retries; retries++ {
			rpcClient, err := c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
			if err != nil || rpcClient == nil {
				continue
			}

			err = rpcClient.Call(serviceMethod, args, reply)
			if err == nil {
				return nil
			}
		}
	} else if c.FailMode == Failtry {
		for retries := 0; retries < c.Retries; retries++ {
			if rpcClient == nil {
				rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
			}
			if rpcClient != nil {
				err = rpcClient.Call(serviceMethod, args, reply)
				if err == nil {
					return nil
				}
			}
		}
	}

	return
}

func (c *Client) clientBroadCast(serviceMethod string, args interface{}, reply interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		return nil
	}

	l := len(rpcClients)
	done := make(chan *rpc.Call, l)
	for _, rpcClient := range rpcClients {
		rpcClient.Go(serviceMethod, args, reply, done)
	}

	for l > 0 {
		call := <-done
		if call == nil || call.Error != nil {
			return errors.New("some clients return Error")
		}
		reply = call.Reply
		l--
	}

	return nil
}

func (c *Client) clientForking(serviceMethod string, args interface{}, reply interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		return nil
	}

	l := len(rpcClients)
	done := make(chan *rpc.Call, l)
	for _, rpcClient := range rpcClients {
		rpcClient.Go(serviceMethod, args, reply, done)
	}

	for l > 0 {
		call := <-done
		if call != nil && call.Error == nil {
			reply = call.Reply
			return nil
		}
		if call == nil {
			break
		}
		l--
	}

	return errors.New("all clients return Error")
}

//Go invokes the function asynchronously. It returns the Call structure representing the invocation.
//The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
func (c *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	rpcClient, _ := c.ClientSelector.Select(c.ClientCodecFunc)

	return rpcClient.Go(serviceMethod, args, reply, done)
}

// Auth sets Authorization info
func (c *Client) Auth(authorization, tag string) error {
	p := NewAuthorizationClientPlugin(authorization, tag)
	return c.PluginContainer.Add(p)
}

type clientCodecWrapper struct {
	rpc.ClientCodec
	PluginContainer IClientPluginContainer
	Timeout         time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	Conn            net.Conn
}

// newClientCodecWrapper wraps a rpc.ServerCodec.
func newClientCodecWrapper(pc IClientPluginContainer, c rpc.ClientCodec, conn net.Conn) *clientCodecWrapper {
	return &clientCodecWrapper{ClientCodec: c, PluginContainer: pc, Conn: conn}
}

func (w *clientCodecWrapper) ReadRequestHeader(r *rpc.Response) error {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetReadDeadline(time.Now().Add(w.ReadTimeout))
	}

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
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetWriteDeadline(time.Now().Add(w.WriteTimeout))
	}

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
