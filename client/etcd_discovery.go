// +build etcd

package client

import (
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/smallnest/rpcx/log"
)

func init() {
	etcd.Register()
}

// EtcdDiscovery is a etcd service discovery.
// It always returns the registered servers in etcd.
type EtcdDiscovery struct {
	basePath string
	kv       store.Store
	pairs    []*KVPair
	chans    []chan []*KVPair
}

// NewEtcdDiscovery returns a new EtcdDiscovery.
func NewEtcdDiscovery(basePath string, etcdAddr []string, options *store.Config) ServiceDiscovery {
	kv, err := libkv.NewStore(store.ETCD, etcdAddr, options)
	if err != nil {
		log.Infof("cannot create store: %v", err)
		panic(err)
	}

	return NewEtcdDiscoveryStore(basePath, kv)
}

// NewEtcdDiscoveryStore return a new EtcdDiscovery with specified store.
func NewEtcdDiscoveryStore(basePath string, kv store.Store) ServiceDiscovery {
	d := &EtcdDiscovery{basePath: basePath, kv: kv}
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
func (d EtcdDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *EtcdDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *EtcdDiscovery) watch() {
	c, err := d.kv.WatchTree(d.basePath, nil)
	if err != nil {
		log.Fatalf("can not watchtree: %s: %v", d.basePath, err)
		return
	}

	for ps := range c {
		var pairs []*KVPair // latest servers
		prefix := d.basePath + "/"
		for _, p := range ps {
			k := strings.TrimPrefix(p.Key, prefix)
			pairs = append(pairs, &KVPair{Key: k, Value: string(p.Value)})
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
