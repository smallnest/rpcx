package client

import (
	"sync"
	"time"

	"github.com/smallnest/rpcx/log"
)

// MultipleServersDiscovery is a multiple servers service discovery.
// It always returns the current servers and uses can change servers dynamically.
type MultipleServersDiscovery struct {
	pairs []*KVPair
	chans []chan []*KVPair
	mu    sync.Mutex
}

// NewMultipleServersDiscovery returns a new MultipleServersDiscovery.
func NewMultipleServersDiscovery(pairs []*KVPair) ServiceDiscovery {
	return &MultipleServersDiscovery{
		pairs: pairs,
	}
}

// Clone clones this ServiceDiscovery with new servicePath.
func (d MultipleServersDiscovery) Clone(servicePath string) ServiceDiscovery {
	return &d
}

// GetServices returns the configured server
func (d MultipleServersDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *MultipleServersDiscovery) WatchService() chan []*KVPair {
	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *MultipleServersDiscovery) RemoveWatcher(ch chan []*KVPair) {
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

// Update is used to update servers at runtime.
func (d *MultipleServersDiscovery) Update(pairs []*KVPair) {
	for _, ch := range d.chans {
		ch := ch
		go func() {
			defer func() {
				if r := recover(); r != nil {

				}
			}()
			select {
			case ch <- pairs:
			case <-time.After(time.Minute):
				log.Warn("chan is full and new change has been dropped")
			}
		}()
	}
}

func (d *MultipleServersDiscovery) Close() {

}
