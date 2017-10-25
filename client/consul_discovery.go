// +build consul

package client

import (
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	"github.com/smallnest/rpcx/log"
)

func init() {
	consul.Register()
}

// ConsulDiscovery is a consul service discovery.
// It always returns the registered servers in consul.
type ConsulDiscovery struct {
	basePath string
	kv       store.Store
	pairs    []*KVPair
	chans    []chan []*KVPair
}

// NewConsulDiscovery returns a new ConsulDiscovery.
func NewConsulDiscovery(basePath string, consulAddr []string, options *store.Config) ServiceDiscovery {
	kv, err := libkv.NewStore(store.CONSUL, consulAddr, options)
	if err != nil {
		log.Infof("cannot create store: %v", err)
		panic(err)
	}

	return NewConsulDiscoveryStore(basePath, kv)
}

// NewConsulDiscoveryStore returns a new ConsulDiscovery with specified store.
func NewConsulDiscoveryStore(basePath string, kv store.Store) ServiceDiscovery {
	if basePath[0] == '/' {
		basePath = basePath[1:]
	}
	d := &ConsulDiscovery{basePath: basePath, kv: kv}
	go d.watch()

	ps, err := kv.List(basePath)
	if err != nil {
		log.Infof("cannot get services of from registry: %v", basePath, err)
		panic(err)
	}

	var pairs []*KVPair
	prefix := d.basePath + "/"
	for _, p := range ps {
		k := strings.TrimPrefix(p.Key, prefix)
		pairs = append(pairs, &KVPair{Key: k, Value: string(p.Value)})
	}
	d.pairs = pairs
	return d
}

// GetServices returns the servers
func (d ConsulDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *ConsulDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *ConsulDiscovery) watch() {
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
