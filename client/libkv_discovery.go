package client

import (
	"strings"
	"time"

	"github.com/docker/libkv/store"
	"github.com/smallnest/rpcx/log"
)

// LibkvDiscovery is a libkv service discovery.
// It always returns the registered servers in etcd.
type LibkvDiscovery struct {
	basePath string
	kv       store.Store
	pairs    []*KVPair
	chans    []chan []*KVPair
}

// NewLibkvDiscovery returns a new LibkvDiscovery.
// Must register the store before use it.
func NewLibkvDiscovery(basePath string, kv store.Store) ServiceDiscovery {
	d := &LibkvDiscovery{basePath: basePath, kv: kv}
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
func (d LibkvDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d LibkvDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d LibkvDiscovery) watch() {
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
