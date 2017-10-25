// +build zookeeper

package client

import (
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/zookeeper"
	"github.com/smallnest/rpcx/log"
)

func init() {
	zookeeper.Register()
}

// ZookeeperDiscovery is a zoopkeer service discovery.
// It always returns the registered servers in zookeeper.
type ZookeeperDiscovery struct {
	basePath string
	kv       store.Store
	pairs    []*KVPair
	chans    []chan []*KVPair
}

// NewZookeeperDiscovery returns a new ZookeeperDiscovery.
func NewZookeeperDiscovery(basePath string, zkAddr []string, options *store.Config) ServiceDiscovery {
	kv, err := libkv.NewStore(store.ZK, zkAddr, options)
	if err != nil {
		log.Infof("cannot create store: %v", err)
		panic(err)
	}

	return NewZookeeperDiscoveryWithStore(basePath, kv)
}

// NewZookeeperDiscoveryWithStore returns a new ZookeeperDiscovery with specified store.
func NewZookeeperDiscoveryWithStore(basePath string, kv store.Store) ServiceDiscovery {
	if basePath[0] == '/' {
		basePath = basePath[1:]
	}
	d := &ZookeeperDiscovery{basePath: basePath, kv: kv}
	go d.watch()

	ps, err := kv.List(basePath)
	if err != nil {
		log.Infof("cannot get services of from registry: %v", basePath, err)
		panic(err)
	}

	var pairs []*KVPair
	for _, p := range ps {
		pairs = append(pairs, &KVPair{Key: p.Key, Value: string(p.Value)})
	}
	d.pairs = pairs
	return d
}

// GetServices returns the servers
func (d ZookeeperDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *ZookeeperDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *ZookeeperDiscovery) watch() {
	c, err := d.kv.WatchTree(d.basePath, nil)
	if err != nil {
		log.Fatalf("can not watchtree: %s: %v", d.basePath, err)
		return
	}

	for ps := range c {
		var pairs []*KVPair // latest servers
		for _, p := range ps {
			pairs = append(pairs, &KVPair{Key: p.Key, Value: string(p.Value)})
		}
		d.pairs = pairs

		for _, ch := range d.chans {
			ch := ch
			go func() {
				select {
				case ch <- pairs:
				case <-time.After(time.Minute):
					log.Warn("chan is full and new change has ben dropped")
				}
			}()
		}
	}
}
