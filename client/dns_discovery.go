package client

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/smallnest/rpcx/log"
)

// DNSDiscovery is based on DNS a record.
// You must set port and network info when you create the DNSDiscovery.
type DNSDiscovery struct {
	domain  string
	network string
	port    int
	d       time.Duration

	pairsMu sync.RWMutex
	pairs   []*KVPair
	chans   []chan []*KVPair

	mu sync.Mutex

	filter ServiceDiscoveryFilter

	stopCh chan struct{}
}

// NewPeer2PeerDiscovery returns a new Peer2PeerDiscovery.
func NewDNSDiscovery(domain string, network string, port int, d time.Duration) (*DNSDiscovery, error) {
	discovery := &DNSDiscovery{domain: domain, network: network, port: port, d: d}
	discovery.lookup()
	go discovery.watch()
	return discovery, nil
}

// Clone clones this ServiceDiscovery with new servicePath.
func (d *DNSDiscovery) Clone(servicePath string) (ServiceDiscovery, error) {
	return NewDNSDiscovery(d.domain, d.network, d.port, d.d)
}

// SetFilter sets the filer.
func (d *DNSDiscovery) SetFilter(filter ServiceDiscoveryFilter) {
	d.filter = filter
}

// GetServices returns the static server
func (d *DNSDiscovery) GetServices() []*KVPair {
	d.pairsMu.RLock()
	defer d.pairsMu.RUnlock()
	return d.pairs
}

// WatchService returns a nil chan.
func (d *DNSDiscovery) WatchService() chan []*KVPair {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *DNSDiscovery) RemoveWatcher(ch chan []*KVPair) {
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

func (d *DNSDiscovery) lookup() {
	var pairs []*KVPair // latest servers

	ips, err := net.LookupIP(d.domain)
	if err != nil {
		log.Errorf("failed to lookup %s: %v", d.domain, err)
		return
	}

	for _, ip := range ips {
		pair := &KVPair{Key: fmt.Sprintf("%s@%s:%d", d.network, ip.String(), d.port)}
		if d.filter != nil && !d.filter(pair) {
			continue
		}
		pairs = append(pairs, pair)
	}

	if len(pairs) > 0 {
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].Key < pairs[j].Key
		})
	}

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
				log.Warn("chan is full and new change has been dropped")
			}
		}()
	}
	d.mu.Unlock()
}

func (d *DNSDiscovery) watch() {
	tick := time.NewTicker(d.d)
	defer tick.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-tick.C:
			d.lookup()
		}
	}
}

func (d *DNSDiscovery) Close() {
	close(d.stopCh)
}
