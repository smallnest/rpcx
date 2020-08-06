package client

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/smallnest/rpcx/v5/share"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/smallnest/rpcx/v5/protocol"
)

// OneClient wraps servicesPath and XClients.
// Users can use a shared oneclient to access multiple services.
type OneClient struct {
	xclients map[string]XClient
	mu       sync.RWMutex

	failMode   FailMode
	selectMode SelectMode
	discovery  ServiceDiscovery
	option     Option

	selectors map[string]Selector
	Plugins   PluginContainer
	latitude  float64
	longitude float64
	auth      string

	serverMessageChan chan<- *protocol.Message
}

// NewOneClient creates a OneClient that supports service discovery and service governance.
func NewOneClient(failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option) *OneClient {
	return &OneClient{
		failMode:   failMode,
		selectMode: selectMode,
		discovery:  discovery,
		option:     option,
		xclients:   make(map[string]XClient),
		selectors:  make(map[string]Selector),
	}
}

// NewBidirectionalOneClient creates a new xclient that can receive notifications from servers.
func NewBidirectionalOneClient(failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option, serverMessageChan chan<- *protocol.Message) *OneClient {
	return &OneClient{
		failMode:          failMode,
		selectMode:        selectMode,
		discovery:         discovery,
		option:            option,
		xclients:          make(map[string]XClient),
		selectors:         make(map[string]Selector),
		serverMessageChan: serverMessageChan,
	}
}

// SetSelector sets customized selector by users.
func (c *OneClient) SetSelector(servicePath string, s Selector) {
	c.mu.Lock()
	c.selectors[servicePath] = s
	if xclient, ok := c.xclients[servicePath]; ok {
		xclient.SetSelector(s)
	}
	c.mu.Unlock()
}

// SetPlugins sets client's plugins.
func (c *OneClient) SetPlugins(plugins PluginContainer) {
	c.Plugins = plugins
	c.mu.RLock()
	for _, v := range c.xclients {
		v.SetPlugins(plugins)
	}
	c.mu.RUnlock()
}

func (c *OneClient) GetPlugins() PluginContainer {
	return c.Plugins
}

// ConfigGeoSelector sets location of client's latitude and longitude,
// and use newGeoSelector.
func (c *OneClient) ConfigGeoSelector(latitude, longitude float64) {
	c.selectMode = Closest
	c.latitude = latitude
	c.longitude = longitude

	c.mu.RLock()
	for _, v := range c.xclients {
		v.ConfigGeoSelector(latitude, longitude)
	}
	c.mu.RUnlock()
}

// Auth sets s token for Authentication.
func (c *OneClient) Auth(auth string) {
	c.auth = auth
	c.mu.RLock()
	for _, v := range c.xclients {
		v.Auth(auth)
	}
	c.mu.RUnlock()
}

// Go invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
// It does not use FailMode.
func (c *OneClient) Go(ctx context.Context, servicePath string, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error) {
	c.mu.RLock()
	xclient := c.xclients[servicePath]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[servicePath]
		if xclient == nil {
			xclient, err = c.newXClient(servicePath)
			c.xclients[servicePath] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}

	return xclient.Go(ctx, serviceMethod, args, reply, done)
}

func (c *OneClient) newXClient(servicePath string) (xclient XClient, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	if c.serverMessageChan == nil {
		xclient = NewXClient(servicePath, c.failMode, c.selectMode, c.discovery.Clone(servicePath), c.option)
	} else {
		xclient = NewBidirectionalXClient(servicePath, c.failMode, c.selectMode, c.discovery.Clone(servicePath), c.option, c.serverMessageChan)
	}

	if c.Plugins != nil {
		xclient.SetPlugins(c.Plugins)
	}

	if s, ok := c.selectors[servicePath]; ok {
		xclient.SetSelector(s)
	}

	if c.selectMode == Closest {
		xclient.ConfigGeoSelector(c.latitude, c.longitude)
	}

	if c.auth != "" {
		xclient.Auth(c.auth)
	}

	return xclient, err
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// It handles errors base on FailMode.
func (c *OneClient) Call(ctx context.Context, servicePath string, serviceMethod string, args interface{}, reply interface{}) error {
	c.mu.RLock()
	xclient := c.xclients[servicePath]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[servicePath]
		if xclient == nil {
			xclient, err = c.newXClient(servicePath)
			c.xclients[servicePath] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}

	return xclient.Call(ctx, serviceMethod, args, reply)
}

func (c *OneClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	servicePath := r.ServicePath

	c.mu.RLock()
	xclient := c.xclients[servicePath]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[servicePath]
		if xclient == nil {
			xclient, err = c.newXClient(servicePath)
			c.xclients[servicePath] = xclient
		}
		c.mu.Unlock()

		if err != nil {
			return nil, nil, err
		}
	}

	return xclient.SendRaw(ctx, r)
}

// Broadcast sends requests to all servers and Success only when all servers return OK.
// FailMode and SelectMode are meanless for this method.
// Please set timeout to avoid hanging.
func (c *OneClient) Broadcast(ctx context.Context, servicePath string, serviceMethod string, args interface{}, reply interface{}) error {
	c.mu.RLock()
	xclient := c.xclients[servicePath]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[servicePath]
		if xclient == nil {
			xclient, err = c.newXClient(servicePath)
			c.xclients[servicePath] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}

	return xclient.Broadcast(ctx, serviceMethod, args, reply)
}

// Fork sends requests to all servers and Success once one server returns OK.
// FailMode and SelectMode are meanless for this method.
func (c *OneClient) Fork(ctx context.Context, servicePath string, serviceMethod string, args interface{}, reply interface{}) error {
	c.mu.RLock()
	xclient := c.xclients[servicePath]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[servicePath]
		if xclient == nil {
			xclient, err = c.newXClient(servicePath)
			c.xclients[servicePath] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}

	return xclient.Fork(ctx, serviceMethod, args, reply)
}

func (c *OneClient) SendFile(ctx context.Context, fileName string, rateInBytesPerSecond int64) error {
	c.mu.RLock()
	xclient := c.xclients[share.SendFileServiceName]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[share.SendFileServiceName]
		if xclient == nil {
			xclient, err = c.newXClient(share.SendFileServiceName)
			c.xclients[share.SendFileServiceName] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}

	return xclient.SendFile(ctx, fileName, rateInBytesPerSecond)
}

func (c *OneClient) DownloadFile(ctx context.Context, requestFileName string, saveTo io.Writer) error {
	c.mu.RLock()
	xclient := c.xclients[share.SendFileServiceName]
	c.mu.RUnlock()

	if xclient == nil {
		var err error
		c.mu.Lock()
		xclient = c.xclients[share.SendFileServiceName]
		if xclient == nil {
			xclient, err = c.newXClient(share.SendFileServiceName)
			c.xclients[share.SendFileServiceName] = xclient
		}
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}

	return xclient.DownloadFile(ctx, requestFileName, saveTo)
}

// Close closes all xclients and its underlying connnections to services.
func (c *OneClient) Close() error {
	var result error

	c.mu.RLock()
	for _, v := range c.xclients {
		err := v.Close()
		if err != nil {
			result = multierror.Append(result, err)
		}
	}
	c.mu.RUnlock()

	return result
}
