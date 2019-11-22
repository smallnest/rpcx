package client

// InprocessDiscovery is a in-process service discovery.
// Clients and servers are in one process and communicate without tcp/udp.
type InprocessDiscovery struct {
}

// NewInprocessDiscovery returns a new InprocessDiscovery.
func NewInprocessDiscovery() ServiceDiscovery {
	return &InprocessDiscovery{}
}

// Clone clones this ServiceDiscovery with new servicePath.
func (d *InprocessDiscovery) Clone(servicePath string) ServiceDiscovery {
	return d
}

// SetFilter sets the filer.
func (d *InprocessDiscovery) SetFilter(filter ServiceDiscoveryFilter) {

}

// GetServices returns the static server
func (d *InprocessDiscovery) GetServices() []*KVPair {
	return []*KVPair{&KVPair{Key: "inprocess@127.0.0.1:0", Value: ""}}
}

// WatchService returns a nil chan.
func (d InprocessDiscovery) WatchService() chan []*KVPair {
	return nil
}

func (d InprocessDiscovery) RemoveWatcher(ch chan []*KVPair) {
}

func (d *InprocessDiscovery) Close() {

}
