package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	ex "github.com/smallnest/rpcx/errors"
	"github.com/smallnest/rpcx/protocol"
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
