package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juju/ratelimit"
	"golang.org/x/sync/singleflight"

	ex "github.com/smallnest/rpcx/errors"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
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
	Oneshot(ctx context.Context, serviceMethod string, args interface{}) error
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
	c.mu.Lock()
	s.UpdateServer(c.servers)
	c.selector = s
	c.mu.Unlock()
}

// KVPair contains a key and a string.
type KVPair struct {
	Key   string
	Value string
}

type xClient struct {
	failMode     FailMode
	selectMode   SelectMode
	cachedClient map[string]RPCClient
	breakers     sync.Map
	servicePath  string
	option       Option

	mu              sync.RWMutex
	servers         map[string]string
	unstableServers map[string]time.Time // 一些服务器重启，如果和它们建立链接，可能会耗费非常长的时间，这里记录袭来需要临时屏蔽
	discovery       ServiceDiscovery
	selector        Selector
	stickyRPCClient RPCClient
	stickyK         string

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
		failMode:        failMode,
		selectMode:      selectMode,
		discovery:       discovery,
		servicePath:     servicePath,
		cachedClient:    make(map[string]RPCClient),
		unstableServers: make(map[string]time.Time),
		option:          option,
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
		unstableServers:   make(map[string]time.Time),
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

	var replyOnce sync.Once

	ctx = setServerTimeout(ctx)
	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
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
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}

			if e == nil && reply != nil && clonedReply != nil {
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
			}
		}()
	}

check:
	for {
		select {
		case result := <-done:
			l--
			if l == 0 || !result { // all returns or some one returns an error
				break check
			}
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return err.ErrorOrNil()
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

	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
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

	var replyOnce sync.Once

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
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
			}
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
		}()
	}

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
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return err.ErrorOrNil()
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

	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
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

	var replyOnce sync.Once

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
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
			if e == nil && reply != nil && clonedReply != nil {
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
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

check:
	for {
		select {
		case <-done:
			l--
			if l == 0 { // all returns or some one returns an error
				break check
			}
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return receipts, err.ErrorOrNil()
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
			err = ctx.Err()
			break loop
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
			err = ctx.Err()
			break loop
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
