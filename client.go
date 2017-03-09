package rpcx

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/rpc"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/saiser/rpcx/log"
	kcp "github.com/xtaci/kcp-go"
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
	"Closest",
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
	//handle failed client
	HandleFailedClient(client *rpc.Client)
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

func (s *DirectClientSelector) HandleFailedClient(client *rpc.Client) {
	client.Close()
	s.rpcClient = nil // reset
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
	Block           kcp.BlockCrypt
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
		return c.clientBroadCast(serviceMethod, args, &reply)
	}
	if c.FailMode == Forking {
		return c.clientForking(serviceMethod, args, &reply)
	}

	var rpcClient *rpc.Client
	//select a rpc.Client and call
	rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
	//selected
	if err == nil && rpcClient != nil {
		if err = rpcClient.Call(serviceMethod, args, reply); err == nil {
			return //call successful
		}

		log.Errorf("failed to call: %v", err)
		c.ClientSelector.HandleFailedClient(rpcClient)
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

			log.Errorf("failed to call: %v", err)
			c.ClientSelector.HandleFailedClient(rpcClient)

		}
	} else if c.FailMode == Failtry {
		for retries := 0; retries < c.Retries; retries++ {
			if rpcClient == nil {
				if rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args); err != nil {
					log.Errorf("failed to select a client: %v", err)
				}
			}

			if rpcClient != nil {
				err = rpcClient.Call(serviceMethod, args, reply)
				if err == nil {
					return nil
				}

				log.Errorf("failed to call: %v", err)
				c.ClientSelector.HandleFailedClient(rpcClient)
			}
		}
	}

	return
}

func (c *Client) clientBroadCast(serviceMethod string, args interface{}, reply *interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		log.Infof("no any client is available")
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
			if call != nil {
				log.Warnf("failed to call: %v", call.Error)
			}
			return errors.New("some clients return Error")
		}
		*reply = call.Reply
		l--
	}

	return nil
}

func (c *Client) clientForking(serviceMethod string, args interface{}, reply *interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		log.Infof("no any client is available")
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
			*reply = call.Reply
			return nil
		}
		if call == nil {
			break
		}
		if call.Error != nil {
			log.Warnf("failed to call: %v", call.Error)
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
		log.Errorf("failed to DoPreReadResponseHeader: %v", err)
		return err
	}

	err = w.ClientCodec.ReadResponseHeader(r)
	if err != nil {
		log.Errorf("failed to ReadResponseHeader: %v", err)
		return err
	}

	//post
	return w.PluginContainer.DoPostReadResponseHeader(r)
}

func (w *clientCodecWrapper) ReadRequestBody(body interface{}) error {
	//pre
	err := w.PluginContainer.DoPreReadResponseBody(body)
	if err != nil {
		log.Errorf("failed to DoPreReadResponseBody: %v", err)
		return err
	}

	err = w.ClientCodec.ReadResponseBody(body)
	if err != nil {
		log.Errorf("failed to ReadResponseBody: %v", err)
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
		log.Errorf("failed to DoPreWriteRequest: %v", err)
		return err
	}

	err = w.ClientCodec.WriteRequest(r, body)
	if err != nil {
		log.Errorf("failed to WriteRequest: %v", err)
		return err
	}

	//post
	return w.PluginContainer.DoPostWriteRequest(r, body)
}

func (w *clientCodecWrapper) Close() error {
	return w.ClientCodec.Close()
}
