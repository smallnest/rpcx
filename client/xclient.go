package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juju/ratelimit"
	ex "github.com/smallnest/rpcx/errors"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"golang.org/x/sync/singleflight"
)

const (
	FileTransferBufferSize = 1024
)

var (
	// ErrXClientShutdown xclient is shutdown.
	ErrXClientShutdown = errors.New("xClient is shut down")
	// ErrXClientNoServer selector can't found one server.
	ErrXClientNoServer = errors.New("can not found any server")
	// ErrServerUnavailable selected server is unavailable.
	ErrServerUnavailable = errors.New("selected server is unavailable")
)

// Receipt represents the result of the service returned.
type Receipt struct {
	Address string
	Reply   interface{}
	Error   error
}

// XClient is an interface that used by client with service discovery and service governance.
// One XClient is used only for one service. You should create multiple XClient for multiple services.
type XClient interface {
	SetPlugins(plugins PluginContainer)
	GetPlugins() PluginContainer
	SetSelector(s Selector)
	ConfigGeoSelector(latitude, longitude float64)
	Auth(auth string)

	Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	Broadcast(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	Fork(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	Inform(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) ([]Receipt, error)
	SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error)
	SendFile(ctx context.Context, fileName string, rateInBytesPerSecond int64, meta map[string]string) error
	DownloadFile(ctx context.Context, requestFileName string, saveTo io.Writer, meta map[string]string) error
	Stream(ctx context.Context, meta map[string]string) (net.Conn, error)
	Close() error
}

// SetSelector sets customized selector by users.
func (c *xClient) SetSelector(s Selector) {
	c.mu.RLock()
	s.UpdateServer(c.servers)
	c.mu.RUnlock()

	c.selector = s
}

// KVPair contains a key and a string.
type KVPair struct {
	Key   string
	Value string
}

// ServiceDiscoveryFilter can be used to filter services with customized logics.
// Servers can register its services but clients can use the customized filter to select some services.
// It returns true if ServiceDiscovery wants to use this service, otherwise it returns false.
type ServiceDiscoveryFilter func(kvp *KVPair) bool

// ServiceDiscovery defines ServiceDiscovery of zookeeper, etcd and consul
type ServiceDiscovery interface {
	GetServices() []*KVPair
	WatchService() chan []*KVPair
	RemoveWatcher(ch chan []*KVPair)
	Clone(servicePath string) (ServiceDiscovery, error)
	SetFilter(ServiceDiscoveryFilter)
	Close()
}

type xClient struct {
	failMode     FailMode
	selectMode   SelectMode
	cachedClient map[string]RPCClient
	breakers     sync.Map
	servicePath  string
	option       Option

	mu        sync.RWMutex
	servers   map[string]string
	discovery ServiceDiscovery
	selector  Selector

	slGroup singleflight.Group

	isShutdown bool

	// auth is a string for Authentication, for example, "Bearer mF_9.B5f-4.1JqM"
	auth string

	Plugins PluginContainer

	ch chan []*KVPair

	serverMessageChan chan<- *protocol.Message
}

// NewXClient creates a XClient that supports service discovery and service governance.
func NewXClient(servicePath string, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option) XClient {
	client := &xClient{
		failMode:     failMode,
		selectMode:   selectMode,
		discovery:    discovery,
		servicePath:  servicePath,
		cachedClient: make(map[string]RPCClient),
		option:       option,
	}

	pairs := discovery.GetServices()
	sort.Slice(pairs, func(i, j int) bool {
		return strings.Compare(pairs[i].Key, pairs[j].Key) <= 0
	})
	servers := make(map[string]string, len(pairs))
	for _, p := range pairs {
		servers[p.Key] = p.Value
	}
	filterByStateAndGroup(client.option.Group, servers)

	client.servers = servers
	if selectMode != Closest && selectMode != SelectByUser {
		client.selector = newSelector(selectMode, servers)
	}

	client.Plugins = &pluginContainer{}

	ch := client.discovery.WatchService()
	if ch != nil {
		client.ch = ch
		go client.watch(ch)
	}

	return client
}

// NewBidirectionalXClient creates a new xclient that can receive notifications from servers.
func NewBidirectionalXClient(servicePath string, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option, serverMessageChan chan<- *protocol.Message) XClient {
	client := &xClient{
		failMode:          failMode,
		selectMode:        selectMode,
		discovery:         discovery,
		servicePath:       servicePath,
		cachedClient:      make(map[string]RPCClient),
		option:            option,
		serverMessageChan: serverMessageChan,
	}

	pairs := discovery.GetServices()
	sort.Slice(pairs, func(i, j int) bool {
		return strings.Compare(pairs[i].Key, pairs[j].Key) <= 0
	})
	servers := make(map[string]string, len(pairs))
	for _, p := range pairs {
		servers[p.Key] = p.Value
	}
	filterByStateAndGroup(client.option.Group, servers)
	client.servers = servers
	if selectMode != Closest && selectMode != SelectByUser {
		client.selector = newSelector(selectMode, servers)
	}

	client.Plugins = &pluginContainer{}

	ch := client.discovery.WatchService()
	if ch != nil {
		client.ch = ch
		go client.watch(ch)
	}

	return client
}

// SetPlugins sets client's plugins.
func (c *xClient) SetPlugins(plugins PluginContainer) {
	c.Plugins = plugins
}

func (c *xClient) GetPlugins() PluginContainer {
	return c.Plugins
}

// ConfigGeoSelector sets location of client's latitude and longitude,
// and use newGeoSelector.
func (c *xClient) ConfigGeoSelector(latitude, longitude float64) {
	c.selector = newGeoSelector(c.servers, latitude, longitude)
	c.selectMode = Closest
}

// Auth sets s token for Authentication.
func (c *xClient) Auth(auth string) {
	c.auth = auth
}

// watch changes of service and update cached clients.
func (c *xClient) watch(ch chan []*KVPair) {
	for pairs := range ch {
		sort.Slice(pairs, func(i, j int) bool {
			return strings.Compare(pairs[i].Key, pairs[j].Key) <= 0
		})
		servers := make(map[string]string, len(pairs))
		for _, p := range pairs {
			servers[p.Key] = p.Value
		}
		c.mu.Lock()
		filterByStateAndGroup(c.option.Group, servers)
		c.servers = servers

		if c.selector != nil {
			c.selector.UpdateServer(servers)
		}

		c.mu.Unlock()
	}
}

func filterByStateAndGroup(group string, servers map[string]string) {
	for k, v := range servers {
		if values, err := url.ParseQuery(v); err == nil {
			if state := values.Get("state"); state == "inactive" {
				delete(servers, k)
			}
			if group != "" && group != values.Get("group") {
				delete(servers, k)
			}
		}
	}
}

// selects a client from candidates base on c.selectMode
func (c *xClient) selectClient(ctx context.Context, servicePath, serviceMethod string, args interface{}) (string, RPCClient, error) {
	c.mu.Lock()
	fn := c.selector.Select
	if c.Plugins != nil {
		fn = c.Plugins.DoWrapSelect(fn)
	}
	k := fn(ctx, servicePath, serviceMethod, args)
	c.mu.Unlock()
	if k == "" {
		return "", nil, ErrXClientNoServer
	}
	client, err := c.getCachedClient(k, servicePath, serviceMethod, args)
	return k, client, err
}

func (c *xClient) getCachedClient(k string, servicePath, serviceMethod string, args interface{}) (RPCClient, error) {
	// TODO: improve the lock
	var client RPCClient
	var needCallPlugin bool
	defer func() {
		if needCallPlugin {
			c.Plugins.DoClientConnected(client.GetConn())
		}
	}()

	if c.isShutdown {
		return nil, errors.New("this xclient is closed")
	}

	// if this client is broken
	breaker, ok := c.breakers.Load(k)
	if ok && !breaker.(Breaker).Ready() {
		return nil, ErrBreakerOpen
	}

	c.mu.Lock()
	client = c.findCachedClient(k, servicePath, serviceMethod)
	if client != nil {
		if !client.IsClosing() && !client.IsShutdown() {
			c.mu.Unlock()
			return client, nil
		}
		c.deleteCachedClient(client, k, servicePath, serviceMethod)
	}

	client = c.findCachedClient(k, servicePath, serviceMethod)
	c.mu.Unlock()

	if client == nil || client.IsShutdown() {
		c.mu.Lock()
		generatedClient, err, _ := c.slGroup.Do(k, func() (interface{}, error) {
			return c.generateClient(k, servicePath, serviceMethod)
		})
		c.mu.Unlock()

		c.slGroup.Forget(k)
		if err != nil {
			return nil, err
		}

		client = generatedClient.(RPCClient)
		if c.Plugins != nil {
			needCallPlugin = true
		}

		client.RegisterServerMessageChan(c.serverMessageChan)

		c.mu.Lock()
		c.setCachedClient(client, k, servicePath, serviceMethod)
		c.mu.Unlock()
	}

	return client, nil
}

func (c *xClient) setCachedClient(client RPCClient, k, servicePath, serviceMethod string) {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok {
		builder.SetCachedClient(client, k, servicePath, serviceMethod)
		return
	}

	c.cachedClient[k] = client
}

func (c *xClient) findCachedClient(k, servicePath, serviceMethod string) RPCClient {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok {
		return builder.FindCachedClient(k, servicePath, serviceMethod)
	}

	return c.cachedClient[k]
}

func (c *xClient) deleteCachedClient(client RPCClient, k, servicePath, serviceMethod string) {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok && client != nil {
		builder.DeleteCachedClient(client, k, servicePath, serviceMethod)
		client.Close()
		return
	}

	delete(c.cachedClient, k)
	if client != nil {
		client.Close()
	}
}

func (c *xClient) removeClient(k, servicePath, serviceMethod string, client RPCClient) {
	c.mu.Lock()
	cl := c.findCachedClient(k, servicePath, serviceMethod)
	if cl == client {
		c.deleteCachedClient(client, k, servicePath, serviceMethod)
	}
	c.mu.Unlock()

	if client != nil {
		client.UnregisterServerMessageChan()
		client.Close()
	}
}

func (c *xClient) generateClient(k, servicePath, serviceMethod string) (client RPCClient, err error) {
	network, addr := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok && builder != nil {
		return builder.GenerateClient(k, servicePath, serviceMethod)
	}

	client = &Client{
		option:  c.option,
		Plugins: c.Plugins,
	}

	var breaker interface{}
	if c.option.GenBreaker != nil {
		breaker, _ = c.breakers.LoadOrStore(k, c.option.GenBreaker())
	}

	err = client.Connect(network, addr)
	if err != nil {
		if breaker != nil {
			breaker.(Breaker).Fail()
		}
		return nil, err
	}
	return client, err
}

func (c *xClient) getCachedClientWithoutLock(k, servicePath, serviceMethod string) (RPCClient, bool, error) {
	var needCallPlugin bool
	client := c.findCachedClient(k, servicePath, serviceMethod)
	if client != nil {
		if !client.IsClosing() && !client.IsShutdown() {
			return client, needCallPlugin, nil
		}
		c.deleteCachedClient(client, k, servicePath, serviceMethod)
	}

	// double check
	client = c.findCachedClient(k, servicePath, serviceMethod)
	if client == nil || client.IsShutdown() {
		generatedClient, err, _ := c.slGroup.Do(k, func() (interface{}, error) {
			return c.generateClient(k, servicePath, serviceMethod)
		})
		c.slGroup.Forget(k)
		if err != nil {
			return nil, needCallPlugin, err
		}

		client = generatedClient.(RPCClient)
		if c.Plugins != nil {
			needCallPlugin = true
		}

		client.RegisterServerMessageChan(c.serverMessageChan)

		c.setCachedClient(client, k, servicePath, serviceMethod)
	}

	return client, needCallPlugin, nil
}

func splitNetworkAndAddress(server string) (string, string) {
	ss := strings.SplitN(server, "@", 2)
	if len(ss) == 1 {
		return "tcp", server
	}

	return ss[0], ss[1]
}

func setServerTimeout(ctx context.Context) context.Context {
	if deadline, ok := ctx.Deadline(); ok {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.ServerTimeout] = fmt.Sprintf("%d", time.Until(deadline).Milliseconds())
	}

	return ctx
}

// Go invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
// It does not use FailMode.
func (c *xClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error) {
	if c.isShutdown {
		return nil, ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)

	if share.Trace {
		log.Debugf("select a client for %s.%s, args: %+v in case of xclient Go", c.servicePath, serviceMethod, args)
	}
	_, client, err := c.selectClient(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		return nil, err
	}
	if share.Trace {
		log.Debugf("selected a client %s for %s.%s, args: %+v in case of xclient Go", client.RemoteAddr(), c.servicePath, serviceMethod, args)
	}
	return client.Go(ctx, c.servicePath, serviceMethod, args, reply, done), nil
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// It handles errors base on FailMode.
func (c *xClient) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}
	ctx = setServerTimeout(ctx)

	if share.Trace {
		log.Debugf("select a client for %s.%s, failMode: %v, args: %+v in case of xclient Call", c.servicePath, serviceMethod, c.failMode, args)
	}

	var err error
	k, client, err := c.selectClient(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		if c.failMode == Failfast || contextCanceled(err) {
			return err
		}
	}

	if share.Trace {
		if client != nil {
			log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", client.RemoteAddr(), c.servicePath, serviceMethod, c.failMode, args)
		} else {
			log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", "nil", c.servicePath, serviceMethod, c.failMode, args)
		}
	}

	var e error
	switch c.failMode {
	case Failtry:
		retries := c.option.Retries
		for retries >= 0 {
			retries--

			if client != nil {
				err = c.wrapCall(ctx, client, serviceMethod, args, reply)
				if err == nil {
					return nil
				}
				if contextCanceled(err) {
					return err
				}
				if _, ok := err.(ServiceError); ok {
					return err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			client, e = c.getCachedClient(k, c.servicePath, serviceMethod, args)
		}
		if err == nil {
			err = e
		}
		return err
	case Failover:
		retries := c.option.Retries
		for retries >= 0 {
			retries--

			if client != nil {
				err = c.wrapCall(ctx, client, serviceMethod, args, reply)
				if err == nil {
					return nil
				}
				if contextCanceled(err) {
					return err
				}
				if _, ok := err.(ServiceError); ok {
					return err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			// select another server
			k, client, e = c.selectClient(ctx, c.servicePath, serviceMethod, args)
		}

		if err == nil {
			err = e
		}
		return err
	case Failbackup:
		ctx, cancelFn := context.WithCancel(ctx)
		defer cancelFn()
		call1 := make(chan *Call, 10)
		call2 := make(chan *Call, 10)

		var reply1, reply2 interface{}

		if reply != nil {
			reply1 = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			reply2 = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
		}

		_, err1 := c.Go(ctx, serviceMethod, args, reply1, call1)

		t := time.NewTimer(c.option.BackupLatency)
		select {
		case <-ctx.Done(): // cancel by context
			err = ctx.Err()
			return err
		case call := <-call1:
			err = call.Error
			if err == nil && reply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply1).Elem())
			}
			return err
		case <-t.C:

		}
		_, err2 := c.Go(ctx, serviceMethod, args, reply2, call2)
		if err2 != nil {
			if uncoverError(err2) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			err = err1
			return err
		}

		select {
		case <-ctx.Done(): // cancel by context
			err = ctx.Err()
		case call := <-call1:
			err = call.Error
			if err == nil && reply != nil && reply1 != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply1).Elem())
			}
		case call := <-call2:
			err = call.Error
			if err == nil && reply != nil && reply2 != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply2).Elem())
			}
		}

		return err
	default: // Failfast
		err = c.wrapCall(ctx, client, serviceMethod, args, reply)
		if err != nil {
			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
		}

		return err
	}
}

func uncoverError(err error) bool {
	if _, ok := err.(ServiceError); ok {
		return false
	}

	if err == context.DeadlineExceeded {
		return false
	}

	if err == context.Canceled {
		return false
	}

	return true
}

func contextCanceled(err error) bool {
	if err == context.DeadlineExceeded {
		return true
	}

	if err == context.Canceled {
		return true
	}

	return false
}

func (c *xClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	if c.isShutdown {
		return nil, nil, ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)

	if share.Trace {
		log.Debugf("select a client for %s.%s, failMode: %v, args: %+v in case of xclient SendRaw", r.ServicePath, r.ServiceMethod, c.failMode, r.Payload)
	}

	var err error
	k, client, err := c.selectClient(ctx, r.ServicePath, r.ServiceMethod, r.Payload)
	if err != nil {
		if c.failMode == Failfast {
			return nil, nil, err
		}
		if contextCanceled(err) {
			return nil, nil, err
		}
		if _, ok := err.(ServiceError); ok {
			return nil, nil, err
		}
	}

	if share.Trace {
		log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", client.RemoteAddr(), r.ServicePath, r.ServiceMethod, c.failMode, r.Payload)
	}

	var e error
	switch c.failMode {
	case Failtry:
		retries := c.option.Retries
		for retries >= 0 {
			retries--
			if client != nil {
				m, payload, err := c.wrapSendRaw(ctx, client, r)
				if err == nil {
					return m, payload, nil
				}
				if contextCanceled(err) {
					return nil, nil, err
				}
				if _, ok := err.(ServiceError); ok {
					return nil, nil, err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
			client, e = c.getCachedClient(k, r.ServicePath, r.ServiceMethod, r.Payload)
		}

		if err == nil {
			err = e
		}
		return nil, nil, err
	case Failover:
		retries := c.option.Retries
		for retries >= 0 {
			retries--
			if client != nil {
				m, payload, err := c.wrapSendRaw(ctx, client, r)
				if err == nil {
					return m, payload, nil
				}
				if contextCanceled(err) {
					return nil, nil, err
				}
				if _, ok := err.(ServiceError); ok {
					return nil, nil, err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
			// select another server
			k, client, e = c.selectClient(ctx, r.ServicePath, r.ServiceMethod, r.Payload)
		}

		if err == nil {
			err = e
		}
		return nil, nil, err

	default: // Failfast
		m, payload, err := c.wrapSendRaw(ctx, client, r)
		if err != nil {
			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
		}

		return m, payload, nil
	}
}

func (c *xClient) wrapCall(ctx context.Context, client RPCClient, serviceMethod string, args interface{}, reply interface{}) error {
	if client == nil {
		return ErrServerUnavailable
	}

	if share.Trace {
		log.Debugf("call a client for %s.%s, args: %+v in case of xclient wrapCall", c.servicePath, serviceMethod, args)
	}

	ctx = share.NewContext(ctx)
	c.Plugins.DoPreCall(ctx, c.servicePath, serviceMethod, args)
	err := client.Call(ctx, c.servicePath, serviceMethod, args, reply)
	c.Plugins.DoPostCall(ctx, c.servicePath, serviceMethod, args, reply, err)

	if share.Trace {
		log.Debugf("called a client for %s.%s, args: %+v, err: %v in case of xclient wrapCall", c.servicePath, serviceMethod, args, err)
	}

	return err
}

// wrapSendRaw wrap SendRaw to support client plugins
func (c *xClient) wrapSendRaw(ctx context.Context, client RPCClient, r *protocol.Message) (map[string]string, []byte, error) {
	if client == nil {
		return nil, nil, ErrServerUnavailable
	}

	if share.Trace {
		log.Debugf("call a client for %s.%s, args: %+v in case of xclient wrapSendRaw", c.servicePath, r.ServiceMethod, r.Payload)
	}

	ctx = share.NewContext(ctx)
	c.Plugins.DoPreCall(ctx, c.servicePath, r.ServiceMethod, r.Payload)
	m, payload, err := client.SendRaw(ctx, r)
	c.Plugins.DoPostCall(ctx, c.servicePath, r.ServiceMethod, r.Payload, nil, err)

	if share.Trace {
		log.Debugf("called a client for %s.%s, args: %+v, err: %v in case of xclient wrapSendRaw", c.servicePath, r.ServiceMethod, r.Payload, err)
	}

	return m, payload, err
}

// Broadcast sends requests to all servers and Success only when all servers return OK.
// FailMode and SelectMode are meanless for this method.
// Please set timeout to avoid hanging.
func (c *xClient) Broadcast(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return ErrXClientNoServer
	}

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		k := k
		client := client
		go func() {
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			done <- (e == nil)
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}

			if e == nil && reply != nil && clonedReply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
			}
		}()
	}

	timeout := time.NewTimer(time.Minute)
check:
	for {
		select {
		case result := <-done:
			l--
			if l == 0 || !result { // all returns or some one returns an error
				break check
			}
		case <-timeout.C:
			err.Append(errors.New(("timeout")))
			break check
		}
	}
	timeout.Stop()

	if err.Error() == "[]" {
		return nil
	}
	return err
}

// Fork sends requests to all servers and Success once one server returns OK.
// FailMode and SelectMode are meanless for this method.
func (c *xClient) Fork(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return ErrXClientNoServer
	}

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		k := k
		client := client
		go func() {
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			if e == nil && reply != nil && clonedReply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
			}
			done <- (e == nil)
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
		}()
	}

	timeout := time.NewTimer(time.Minute)
check:
	for {
		select {
		case result := <-done:
			l--
			if result {
				return nil
			}
			if l == 0 { // all returns or some one returns an error
				break check
			}

		case <-timeout.C:
			err.Append(errors.New(("timeout")))
			break check
		}
	}
	timeout.Stop()

	if err.Error() == "[]" {
		return nil
	}

	return err
}

// Inform sends requests to all servers and returns all results from services.
// FailMode and SelectMode are meanless for this method.
// Please set timeout to avoid hanging.
func (c *xClient) Inform(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) ([]Receipt, error) {
	if c.isShutdown {
		return nil, ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return nil, ErrXClientNoServer
	}

	var receiptsLock sync.Mutex
	var receipts []Receipt

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		k := k
		client := client
		go func() {
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			done <- (e == nil)
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
			if e == nil && reply != nil && clonedReply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
			}

			addr := k
			ss := strings.SplitN(k, "@", 2)
			if len(ss) == 2 {
				addr = ss[1]
			}
			receiptsLock.Lock()

			receipts = append(receipts, Receipt{
				Address: addr,
				Reply:   clonedReply,
				Error:   err,
			})
			receiptsLock.Unlock()
		}()
	}

	timeout := time.NewTimer(time.Minute)
check:
	for {
		select {
		case <-done:
			l--
			if l == 0 { // all returns or some one returns an error
				break check
			}
		case <-timeout.C:
			err.Append(errors.New(("timeout")))
			break check
		}
	}
	timeout.Stop()

	if err.Error() == "[]" {
		return receipts, nil
	}
	return receipts, err
}

// SendFile sends a local file to the server.
// fileName is the path of local file.
// rateInBytesPerSecond can limit bandwidth of sending,  0 means does not limit the bandwidth, unit is bytes / second.
func (c *xClient) SendFile(ctx context.Context, fileName string, rateInBytesPerSecond int64, meta map[string]string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	defer file.Close()

	fi, err := os.Stat(fileName)
	if err != nil {
		return err
	}

	args := share.FileTransferArgs{
		FileName: fi.Name(),
		FileSize: fi.Size(),
		Meta:     meta,
	}

	ctx = setServerTimeout(ctx)

	reply := &share.FileTransferReply{}
	err = c.Call(ctx, "TransferFile", args, reply)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Write(reply.Token)
	if err != nil {
		return err
	}

	var tb *ratelimit.Bucket

	if rateInBytesPerSecond > 0 {
		tb = ratelimit.NewBucketWithRate(float64(rateInBytesPerSecond), rateInBytesPerSecond)
	}

	sendBuffer := make([]byte, FileTransferBufferSize)
loop:
	for {
		select {
		case <-ctx.Done():
		default:
			if tb != nil {
				tb.Wait(FileTransferBufferSize)
			}
			n, err := file.Read(sendBuffer)
			if err != nil {
				if err == io.EOF {
					return nil
				} else {
					return err
				}
			}
			if n == 0 {
				break loop
			}
			_, err = conn.Write(sendBuffer[:n])
			if err != nil {
				if err == io.EOF {
					return nil
				} else {
					return err
				}
			}
		}
	}

	return nil
}

func (c *xClient) DownloadFile(ctx context.Context, requestFileName string, saveTo io.Writer, meta map[string]string) error {
	ctx = setServerTimeout(ctx)

	args := share.DownloadFileArgs{
		FileName: requestFileName,
		Meta:     meta,
	}

	reply := &share.FileTransferReply{}
	err := c.Call(ctx, "DownloadFile", args, reply)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Write(reply.Token)
	if err != nil {
		return err
	}

	buf := make([]byte, FileTransferBufferSize)
	r := bufio.NewReader(conn)
loop:
	for {
		select {
		case <-ctx.Done():
		default:
			n, er := r.Read(buf)
			if n > 0 {
				_, ew := saveTo.Write(buf[0:n])
				if ew != nil {
					err = ew
					break loop
				}
			}
			if er != nil {
				if er != io.EOF {
					err = er
				}
				break loop
			}
		}
	}

	return err
}

// Close closes this client and its underlying connections to services.
func (c *xClient) Close() error {
	var errs []error
	c.mu.Lock()
	c.isShutdown = true
	for k, v := range c.cachedClient {
		e := v.Close()
		if e != nil {
			errs = append(errs, e)
		}

		delete(c.cachedClient, k)

	}
	c.mu.Unlock()

	go func() {
		defer func() {
			recover()
		}()

		c.discovery.RemoveWatcher(c.ch)
		close(c.ch)
	}()

	if len(errs) > 0 {
		return ex.NewMultiError(errs)
	}
	return nil
}

func (c *xClient) Stream(ctx context.Context, meta map[string]string) (net.Conn, error) {
	args := share.StreamServiceArgs{
		Meta: meta,
	}

	ctx = setServerTimeout(ctx)

	reply := &share.StreamServiceReply{}
	err := c.Call(ctx, "Stream", args, reply)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(reply.Token)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
