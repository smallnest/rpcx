package client

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/smallnest/rpcx/log"
)

type serviceMeta struct {
	Service        string `json:"service,omitempty"`
	Meta           string `json:"meta,omitempty"`
	ServiceAddress string `json:"service_address,omitempty"`
}

// MDNSDiscovery is a mdns service discovery.
// It always returns the registered servers in etcd.
type MDNSDiscovery struct {
	Timeout       time.Duration
	WatchInterval time.Duration
	domain        string
	service       string
	pairs         []*KVPair
	chans         []chan []*KVPair
}

// NewMDNSDiscovery returns a new MDNSDiscovery.
// If domain is empty, use "local." in default.
func NewMDNSDiscovery(service string, timeout time.Duration, watchInterval time.Duration, domain string) ServiceDiscovery {
	if domain == "" {
		domain = "local."
	}
	d := &MDNSDiscovery{service: service, Timeout: timeout, WatchInterval: watchInterval, domain: domain}
	var err error
	d.pairs, err = d.browse()
	if err != nil {
		log.Warnf("failed to browse services: %v", err)
	}
	go d.watch()
	return d
}

// GetServices returns the servers
func (d MDNSDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *MDNSDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *MDNSDiscovery) watch() {
	t := time.NewTicker(d.WatchInterval)
	for range t.C {
		pairs, err := d.browse()
		if err == nil {
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
}

func (d *MDNSDiscovery) browse() ([]*KVPair, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Warnf("Failed to initialize resolver: %v", err)
		return nil, err
	}
	entries := make(chan *zeroconf.ServiceEntry)

	var totalServices []*KVPair
	var services []*serviceMeta

	done := make(chan struct{})
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range entries {
			s, _ := url.QueryUnescape(entry.Text[0])
			err := json.Unmarshal([]byte(s), &services)
			if err != nil {
				log.Warnf("Failed to browse: %v", err)
				continue
			}

			for _, sm := range services {
				totalServices = append(totalServices, &KVPair{
					Key:   sm.ServiceAddress,
					Value: sm.Meta,
				})
			}
		}

		close(done)
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), d.Timeout)
	defer cancel()
	err = resolver.Browse(ctx, "_rpcxservices", d.domain, entries)
	if err != nil {
		log.Warnf("Failed to browse: %v", err)
	}

	<-done
	return totalServices, nil
}
