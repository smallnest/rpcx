package client

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
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
// It always returns the registered servers in mdns.
type MDNSDiscovery struct {
	Timeout       time.Duration
	WatchInterval time.Duration
	domain        string
	service       string
	pairsMu       sync.RWMutex
	pairs         []*KVPair
	chans         []chan []*KVPair

	mu sync.Mutex

	filter ServiceDiscoveryFilter

	stopCh chan struct{}
}

// NewMDNSDiscovery returns a new MDNSDiscovery.
// If domain is empty, use "local." in default.
func NewMDNSDiscovery(service string, timeout time.Duration, watchInterval time.Duration, domain string) (*MDNSDiscovery, error) {
	if domain == "" {
		domain = "local."
	}
	d := &MDNSDiscovery{service: service, Timeout: timeout, WatchInterval: watchInterval, domain: domain}
	d.stopCh = make(chan struct{})

	var err error
	d.pairsMu.Lock()
	d.pairs, err = d.browse()
	d.pairsMu.Unlock()
	if err != nil {
		log.Warnf("failed to browse services: %v", err)
	}
	go d.watch()
	return d, nil
}

// Clone clones this ServiceDiscovery with new servicePath.
func (d *MDNSDiscovery) Clone(servicePath string) (ServiceDiscovery, error) {
	return NewMDNSDiscovery(servicePath, d.Timeout, d.WatchInterval, d.domain)
}

// SetFilter sets the filer.
func (d *MDNSDiscovery) SetFilter(filter ServiceDiscoveryFilter) {
	d.filter = filter
}

// GetServices returns the servers
func (d *MDNSDiscovery) GetServices() []*KVPair {
	d.pairsMu.RLock()
	defer d.pairsMu.RUnlock()

	return d.pairs
}

// WatchService returns a nil chan.
func (d *MDNSDiscovery) WatchService() chan []*KVPair {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *MDNSDiscovery) RemoveWatcher(ch chan []*KVPair) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var chans []chan []*KVPair
	for _, c := range d.chans {
		if c == ch {
			continue
		}

		chans = append(chans, c)
	}

	d.chans = chans
}

func (d *MDNSDiscovery) watch() {
	t := time.NewTicker(d.WatchInterval)

	for {
		select {
		case <-d.stopCh:
			t.Stop()
			log.Info("discovery has been closed")
			return
		case <-t.C:
			pairs, err := d.browse()
			if err == nil {
				d.pairsMu.Lock()
				d.pairs = pairs
				d.pairsMu.Unlock()

				d.mu.Lock()
				for _, ch := range d.chans {
					ch := ch
					go func() {
						defer func() {
							recover()
						}()
						select {
						case ch <- pairs:
						case <-time.After(time.Minute):
							log.Warn("chan is full and new change has ben dropped")
						}
					}()
				}
				d.mu.Unlock()
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

				pair := &KVPair{
					Key:   sm.ServiceAddress,
					Value: sm.Meta,
				}
				if d.filter != nil && !d.filter(pair) {
					continue
				}
				totalServices = append(totalServices, pair)
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

func (d *MDNSDiscovery) Close() {
	close(d.stopCh)
}
