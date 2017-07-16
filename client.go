package rpcx

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	msgpackrpc2 "github.com/rpcx-ecosystem/net-rpc-msgpackrpc2"
	"github.com/smallnest/rpcx/core"
	"github.com/smallnest/rpcx/log"
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

//ClientSelector defines an interface to create a  core.Client from cluster or standalone.
type ClientSelector interface {
	//Select returns a new client and it also update current client
	Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*core.Client, error)
	//SetClient set current client
	SetClient(*Client)
	SetSelectMode(SelectMode)
	//AllClients returns all Clients
	AllClients(clientCodecFunc ClientCodecFunc) []*core.Client
	//handle failed client
	HandleFailedClient(client *core.Client)
}

// DirectClientSelector is used to a direct rpc server.
// It don't select a node from service cluster but a specific rpc server.
type DirectClientSelector struct {
	Network, Address string
	DialTimeout      time.Duration
	Client           *Client
	rpcClient        *core.Client
	sync.Mutex
}

//Select returns a rpc client.
func (s *DirectClientSelector) Select(clientCodecFunc ClientCodecFunc, options ...interface{}) (*core.Client, error) {
	s.Lock()
	defer s.Unlock()
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

//AllClients returns  core.Clients to all servers
func (s *DirectClientSelector) AllClients(clientCodecFunc ClientCodecFunc) []*core.Client {
	if s.rpcClient == nil {
		return []*core.Client{}
	}
	return []*core.Client{s.rpcClient}
}

func (s *DirectClientSelector) HandleFailedClient(client *core.Client) {
	s.Lock()
	client.Close()
	if s.rpcClient == client {
		s.rpcClient = nil // reset
	}
	s.Unlock()
}

// ClientCodecFunc is used to create a  core.ClientCodecFunc from net.Conn.
type ClientCodecFunc func(conn io.ReadWriteCloser) core.ClientCodec

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
		ClientCodecFunc: msgpackrpc2.NewClientCodec,
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
func (c *Client) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) (err error) {
	if c.FailMode == Broadcast {
		return c.clientBroadCast(ctx, serviceMethod, args, &reply)
	}
	if c.FailMode == Forking {
		return c.clientForking(ctx, serviceMethod, args, &reply)
	}

	var rpcClient *core.Client
	//select a  core.Client and call
	rpcClient, err = c.ClientSelector.Select(c.ClientCodecFunc, serviceMethod, args)
	//selected
	if err == nil && rpcClient != nil {
		if err = c.wrapCall(rpcClient, ctx, serviceMethod, args, reply); err == nil {
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

			err = c.wrapCall(rpcClient, ctx, serviceMethod, args, reply)
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
				err = c.wrapCall(rpcClient, ctx, serviceMethod, args, reply)
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

func (c *Client) clientBroadCast(ctx context.Context, serviceMethod string, args interface{}, reply *interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		log.Infof("no any client is available")
		return nil
	}

	l := len(rpcClients)
	done := make(chan *core.Call, l)
	for _, rpcClient := range rpcClients {
		if rpcClient.IsShutdown() && Reconnect != nil {
			log.Infof("client has been shutdown")
			c.ClientSelector.HandleFailedClient(rpcClient)
		}
		rpcClient.Go(ctx, serviceMethod, args, reply, done)
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

func (c *Client) clientForking(ctx context.Context, serviceMethod string, args interface{}, reply *interface{}) (err error) {
	rpcClients := c.ClientSelector.AllClients(c.ClientCodecFunc)

	if len(rpcClients) == 0 {
		log.Infof("no any client is available")
		return nil
	}

	l := len(rpcClients)
	done := make(chan *core.Call, l)
	for _, rpcClient := range rpcClients {
		if rpcClient.IsShutdown() && Reconnect != nil {
			log.Infof("client has been shutdown")
			c.ClientSelector.HandleFailedClient(rpcClient)
		}
		rpcClient.Go(ctx, serviceMethod, args, reply, done)
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

func (c *Client) wrapCall(client *core.Client, ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	if client.IsShutdown() && Reconnect != nil {
		log.Infof("client has been shutdown")
		c.ClientSelector.HandleFailedClient(client)
	}

	c.PluginContainer.DoPreCall(ctx, serviceMethod, args, reply)
	err := client.Call(ctx, serviceMethod, args, reply)
	c.PluginContainer.DoPostCall(ctx, serviceMethod, args, reply)
	return err
}

//Go invokes the function asynchronously. It returns the Call structure representing the invocation.
//The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
func (c *Client) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *core.Call) *core.Call {
	rpcClient, _ := c.ClientSelector.Select(c.ClientCodecFunc)

	return rpcClient.Go(ctx, serviceMethod, args, reply, done)
}

// Auth sets Authorization info
func (c *Client) Auth(authorization, tag string) error {
	p := NewAuthorizationClientPlugin(authorization, tag)
	return c.PluginContainer.Add(p)
}

type ClientCodecWrapper struct {
	core.ClientCodec
	ClientCodecFunc
	PluginContainer IClientPluginContainer
	Timeout         time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	Conn            net.Conn
}

// newClientCodecWrapper wraps a  core.ClientCodec.
func newClientCodecWrapper(pc IClientPluginContainer, c core.ClientCodec, conn net.Conn) *ClientCodecWrapper {
	return &ClientCodecWrapper{ClientCodec: c, PluginContainer: pc, Conn: conn}
}

func (w *ClientCodecWrapper) ReadRequestHeader(r *core.Response) error {
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

func (w *ClientCodecWrapper) ReadRequestBody(body interface{}) error {
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

func (w *ClientCodecWrapper) WriteRequest(ctx context.Context, r *core.Request, body interface{}) error {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetWriteDeadline(time.Now().Add(w.WriteTimeout))
	}

	//pre
	err := w.PluginContainer.DoPreWriteRequest(ctx, r, body)
	if err != nil {
		log.Errorf("failed to DoPreWriteRequest: %v", err)
		return err
	}

	err = w.ClientCodec.WriteRequest(ctx, r, body)
	if err != nil {
		log.Errorf("failed to WriteRequest: %v", err)
		return err
	}

	//post
	return w.PluginContainer.DoPostWriteRequest(ctx, r, body)
}

func (w *ClientCodecWrapper) Close() error {
	return w.ClientCodec.Close()
}

// ReconnectFunc recnnect function.
type ReconnectFunc func(client *core.Client, clientAndServer map[string]*core.Client, rpcxClient *Client, dailTimeout time.Duration) bool

// Reconnect strategy. The default reconnect is to reconnect at most 3 times.
var Reconnect ReconnectFunc = reconnect

//try to reconnect
func reconnect(client *core.Client, clientAndServer map[string]*core.Client, rpcxClient *Client, dailTimeout time.Duration) (reconnected bool) {
	var server string
	for k, v := range clientAndServer {
		if v == client {
			server = k
		}
		break
	}

	if server != "" {
		ss := strings.Split(server, "@")

		var clientCodecFunc ClientCodecFunc
		if wrapper, ok := client.Codec().(*ClientCodecWrapper); ok {
			clientCodecFunc = wrapper.ClientCodecFunc
		}

		interval := 100 * time.Millisecond

		if clientCodecFunc != nil {
			for i := 0; i < 3; i++ {
				c, err := NewDirectRPCClient(rpcxClient, clientCodecFunc, ss[0], ss[1], dailTimeout)
				if err == nil {
					// codec := c.Codec()
					// client.SetCodec(codec) //reconnected codec

					// c.Release()
					// c.SetCodec(nil) //free c
					client.Close()
					*client = *c
					log.Warnf("reconnected to server: %s", server)
					return true
				}
				log.Warnf("failed to reconnected to server: %s", server)
				time.Sleep(interval)
				interval = interval * 2
			}

		}
	}

	return false
}
