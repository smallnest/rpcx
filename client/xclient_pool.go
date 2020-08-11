package client

import (
	"sync/atomic"

	"github.com/smallnest/rpcx/protocol"
)

// XClientPool is a xclient pool with fixed size.
// It uses roundrobin algorithm to call its xclients.
// All xclients share the same configurations such as ServiceDiscovery and serverMessageChan.
type XClientPool struct {
	count    uint64
	index    uint64
	xclients []XClient

	servicePath       string
	failMode          FailMode
	selectMode        SelectMode
	discovery         ServiceDiscovery
	option            Option
	serverMessageChan chan<- *protocol.Message
}

// NewXClientPool creates a fixed size XClient pool.
func NewXClientPool(count int, servicePath string, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option) *XClientPool {
	pool := &XClientPool{
		count:       uint64(count),
		xclients:    make([]XClient, count),
		servicePath: servicePath,
		failMode:    failMode,
		selectMode:  selectMode,
		discovery:   discovery,
		option:      option,
	}

	for i := 0; i < count; i++ {
		xclient := NewXClient(servicePath, failMode, selectMode, discovery, option)
		pool.xclients[i] = xclient
	}
	return pool
}

// NewBidirectionalXClientPool creates a BidirectionalXClient pool with fixed size.
func NewBidirectionalXClientPool(count int, servicePath string, failMode FailMode, selectMode SelectMode, discovery ServiceDiscovery, option Option, serverMessageChan chan<- *protocol.Message) *XClientPool {
	pool := &XClientPool{
		count:             uint64(count),
		xclients:          make([]XClient, count),
		servicePath:       servicePath,
		failMode:          failMode,
		selectMode:        selectMode,
		discovery:         discovery,
		option:            option,
		serverMessageChan: serverMessageChan,
	}

	for i := 0; i < count; i++ {
		xclient := NewBidirectionalXClient(servicePath, failMode, selectMode, discovery, option, serverMessageChan)
		pool.xclients[i] = xclient
	}
	return pool
}

// Get returns a xclient.
// It does not remove this xclient from its cache so you don't need to put it back.
// Don't close this xclient because maybe other goroutines are using this xclient.
func (p *XClientPool) Get() XClient {
	i := atomic.AddUint64(&p.index, 1)
	picked := int(i % p.count)
	return p.xclients[picked]
}

// Close this pool.
// Please make sure it won't be used any more.
func (p *XClientPool) Close() {
	for _, c := range p.xclients {
		c.Close()
	}
	p.xclients = nil
}
