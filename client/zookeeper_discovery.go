// +build zookeeper

package client

import (
	"strings"
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

	// -1 means it always retry to watch until zookeeper is ok, 0 means no retry.
	RetriesAfterWatchFailed int
}

// NewZookeeperDiscovery returns a new ZookeeperDiscovery.
func NewZookeeperDiscovery(basePath string, servicePath string, zkAddr []string, options *store.Config) ServiceDiscovery {
	if basePath[0] == '/' {
		basePath = basePath[1:]
	}

	if len(basePath) > 1 && strings.HasSuffix(basePath, "/") {
		basePath = basePath[:len(basePath)-1]
	}

	kv, err := libkv.NewStore(store.ZK, zkAddr, options)
	if err != nil {
		log.Infof("cannot create store: %v", err)
		panic(err)
	}

	return NewZookeeperDiscoveryWithStore(basePath+"/"+servicePath, kv)
}

// NewZookeeperDiscoveryWithStore returns a new ZookeeperDiscovery with specified store.
func NewZookeeperDiscoveryWithStore(basePath string, kv store.Store) ServiceDiscovery {
	if basePath[0] == '/' {
		basePath = basePath[1:]
	}
	d := &ZookeeperDiscovery{basePath: basePath, kv: kv}

	ps, err := kv.List(basePath)
	if err != nil {
		log.Infof("cannot get services of from registry: %v", basePath, err)
		panic(err)
	}

	var pairs = make([]*KVPair, 0, len(ps))
	for _, p := range ps {
		pairs = append(pairs, &KVPair{Key: p.Key, Value: string(p.Value)})
	}
	d.pairs = pairs
	d.RetriesAfterWatchFailed = -1
	go d.watch()

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
	for {
		var err error
		var c <-chan []*store.KVPair
		var tempDelay time.Duration

		retry := d.RetriesAfterWatchFailed
		for d.RetriesAfterWatchFailed == -1 || retry > 0 {
			c, err = d.kv.WatchTree(d.basePath, nil)
			if err != nil {
				if d.RetriesAfterWatchFailed > 0 {
					retry--
				}
				if tempDelay == 0 {
					tempDelay = 1 * time.Second
				} else {
					tempDelay *= 2
				}
				if max := 30 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Warnf("can not watchtree (with retry %d, sleep %v): %s: %v", retry, tempDelay, d.basePath, err)
				time.Sleep(tempDelay)
				continue
			}
			break
		}

		if err != nil {
			log.Fatalf("can not watchtree: %s: %v", d.basePath, err)
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

		log.Warn("chan is closed and will rewatch")
	}
}
