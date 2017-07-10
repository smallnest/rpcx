package clientselector

import (
	"net/rpc"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/core"
	"github.com/smallnest/rpcx/log"
)

// CMap is a goutine-safe/thread-safe map
type CMap struct {
	data map[string]*rpc.Client
	sync.RWMutex
}

func NewCMap() *CMap {
	return &CMap{data: make(map[string]*rpc.Client)}
}

func (s *CMap) Set(key string, value *rpc.Client) {
	s.Lock()
	s.data[key] = value
	s.Unlock()
}
func (s *CMap) Get(key string) *rpc.Client {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}
func (s *CMap) Remove(key string) {
	s.RLock()
	defer s.RUnlock()
	delete(s.data, key)
}

// ReconnectFunc recnnect function.
type ReconnectFunc func(client *core.Client, clientAndServer map[string]*core.Client, rpcxClient *rpcx.Client, dailTimeout time.Duration) bool

// Reconnect strategy. The default reconnect is to reconnect at most 3 times.
var Reconnect ReconnectFunc = reconnect

//try to reconnect
func reconnect(client *core.Client, clientAndServer map[string]*core.Client, rpcxClient *rpcx.Client, dailTimeout time.Duration) (reconnected bool) {
	var server string
	for k, v := range clientAndServer {
		if v == client {
			server = k
		}
		break
	}

	if server != "" {
		ss := strings.Split(server, "@")

		var clientCodecFunc rpcx.ClientCodecFunc
		if wrapper, ok := client.Codec().(*rpcx.ClientCodecWrapper); ok {
			clientCodecFunc = wrapper.ClientCodecFunc
		}

		interval := 100 * time.Millisecond

		if clientCodecFunc != nil {
			for i := 0; i < 3; i++ {
				c, err := rpcx.NewDirectRPCClient(rpcxClient, clientCodecFunc, ss[0], ss[1], dailTimeout)
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
