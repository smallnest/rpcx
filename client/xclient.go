package client

import (
	"context"
	"errors"
	"strings"
	"sync"

	ex "github.com/smallnest/rpcx/errors"
)

// TODO

var (
	// ErrXClientShutdown xclient is shutdown.
	ErrXClientShutdown = errors.New("xClient is shut down")
	// ErrXClientNoServer selector can't found one server.
	ErrXClientNoServer = errors.New("can not found any server")
)

// XClient is an interface that used by client with service discovery and service governance.
// One XClient is used only for one service. You should create multiple XClient for multiple services.
type XClient interface {
	Go(ctx context.Context, args interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, args interface{}, reply interface{}) error
	Broadcast(ctx context.Context, args interface{}, reply interface{}) error
	Fork(ctx context.Context, args interface{}, reply interface{}) error
	Close() error
}

// KVPair contains a key and a string.
type KVPair struct {
	Key   string
	Value string
}

// ServiceDiscovery defines ServiceDiscovery of zookeeper, etcd and consul
type ServiceDiscovery interface {
	GetServices() []*KVPair
	WatchService() chan []*KVPair
}

type xClient struct {
	failMode     FailMode
	selectMode   SelectMode
	cachedClient map[string]*Client

	mu        sync.RWMutex
	servers   map[string]string
	discovery ServiceDiscovery
	selector  Selector

	servicePath   string
	serviceMethod string

	isShutdown bool
}

// NewXClient creates a XClient that supports service discovery and service governance.
func NewXClient(servicePath, serviceMethod string, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery) XClient {
	client := &xClient{
		failMode:      failMode,
		selectMode:    selectMode,
		discovery:     discovery,
		servicePath:   servicePath,
		serviceMethod: serviceMethod,
	}

	go client.watch()

	servers := make(map[string]string)
	pairs := discovery.GetServices()
	for _, p := range pairs {
		servers[p.Key] = p.Value
	}
	client.servers = servers
	client.selector = newSelector(selectMode, servers)

	// TODO init other fields

	return client
}

// watch changes of service and update cached clients.
func (c *xClient) watch() {
	ch := c.discovery.WatchService()
	for pairs := range ch {

		servers := make(map[string]string)
		for _, p := range pairs {
			servers[p.Key] = p.Value
		}
		c.mu.Lock()
		c.servers = servers
		// TODO update other fields
		c.mu.Unlock()
	}
}

// selects a client from candidates base on c.selectMode
func (c *xClient) selectClient(ctx context.Context, servicePath, serviceMethod string) (*Client, error) {
	k := c.selector.Select(ctx, servicePath, serviceMethod)
	if k == "" {
		return nil, ErrXClientNoServer
	}
	return c.getCachedClient(k)
}

func (c *xClient) getCachedClient(k string) (*Client, error) {
	c.mu.RLock()
	client := c.cachedClient[k]
	if client != nil {
		if !client.closing && !client.shutdown {
			c.mu.RUnlock()
			return client, nil
		}
	}

	//double check
	c.mu.Lock()
	client = c.cachedClient[k]
	if client == nil {
		network, addr := splitNetworkAndAddress(k)
		client = &Client{
		// TODO init this client
		}
		err := client.Connect(network, addr)
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		c.cachedClient[k] = client
	}
	c.mu.Unlock()

	return client, nil
}

func splitNetworkAndAddress(server string) (string, string) {
	ss := strings.SplitN(server, "@", 2)
	if len(ss) == 1 {
		return "tcp", server
	}

	return ss[0], ss[1]
}

// Go invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
// It does not use FailMode.
func (c *xClient) Go(ctx context.Context, args interface{}, reply interface{}, done chan *Call) (*Call, error) {
	if c.isShutdown {
		return nil, ErrXClientShutdown
	}
	client, err := c.selectClient(ctx, c.servicePath, c.serviceMethod)
	if err != nil {
		return nil, err
	}
	return client.Go(ctx, c.servicePath, c.serviceMethod, args, reply, done), nil
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// It handles errors base on FailMode.
func (c *xClient) Call(ctx context.Context, args interface{}, reply interface{}) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	client, err := c.selectClient(ctx, c.servicePath, c.serviceMethod)
	if err != nil {
		return err
	}

	return client.Call(ctx, c.servicePath, c.serviceMethod, args, reply)
}

// Broadcast sends requests to all servers and Success only when all servers return OK.
// FailMode and SelectMode are meanless for this method.
func (c *xClient) Broadcast(ctx context.Context, args interface{}, reply interface{}) error {

	return nil
}

// Fork sends requests to all servers and Success once one server returns OK.
// FailMode and SelectMode are meanless for this method.
func (c *xClient) Fork(ctx context.Context, args interface{}, reply interface{}) error {

	return nil
}

// Close closes this client and its underlying connnections to services.
func (c *xClient) Close() error {
	c.isShutdown = true

	var errs []error
	c.mu.Lock()
	for _, v := range c.cachedClient {
		e := v.Close()
		if e != nil {
			errs = append(errs, e)
		}

	}
	c.mu.Unlock()

	if len(errs) > 0 {
		return ex.NewMultiError(errs)
	}
	return nil
}
