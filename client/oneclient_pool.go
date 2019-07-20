package client

import (
	"sync/atomic"

	"github.com/smallnest/rpcx/protocol"
)

// OneClientPool is a oneclient pool with fixed size.
// It uses roundrobin algorithm to call its xclients.
// All oneclients share the same configurations such as ServiceDiscovery and serverMessageChan.
type OneClientPool struct {
	count      uint64
	index      uint64
	oneclients []*OneClient

	servicePath       string
	failMode          FailMode
	selectMode        SelectMode
	discovery         ServiceDiscovery
	option            Option
	serverMessageChan chan<- *protocol.Message
}

// NewOneClientPool creates a fixed size OneClient pool.
func NewOneClientPool(count int, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option) *OneClientPool {
	pool := &OneClientPool{
		count:      uint64(count),
		oneclients: make([]*OneClient, count),
		failMode:   failMode,
		selectMode: selectMode,
		discovery:  discovery,
		option:     option,
	}

	for i := 0; i < count; i++ {
		oneclient := NewOneClient(failMode, selectMode, discovery, option)
		pool.oneclients[i] = oneclient
	}
	return pool
}

// NewBidirectionalOneClientPool creates a BidirectionalOneClient pool with fixed size.
func NewBidirectionalOneClientPool(count int, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option, serverMessageChan chan<- *protocol.Message) *OneClientPool {
	pool := &OneClientPool{
		count:             uint64(count),
		oneclients:        make([]*OneClient, count),
		failMode:          failMode,
		selectMode:        selectMode,
		discovery:         discovery,
		option:            option,
		serverMessageChan: serverMessageChan,
	}

	for i := 0; i < count; i++ {
		oneclient := NewBidirectionalOneClient(failMode, selectMode, discovery, option, serverMessageChan)
		pool.oneclients[i] = oneclient
	}
	return pool
}

// Get returns a OneClient.
// It does not remove this OneClient from its cache so you don't need to put it back.
// Don't close this OneClient because maybe other goroutines are using this OneClient.
func (p OneClientPool) Get() *OneClient {
	i := atomic.AddUint64(&p.index, 1)
	picked := int(i % p.count)
	return p.oneclients[picked]
}

// Close this pool.
// Please make sure it won't be used any more.
func (p OneClientPool) Close() {
	for _, c := range p.oneclients {
		c.Close()
	}
	p.oneclients = nil
}
