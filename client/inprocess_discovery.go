package client

// InpreocessDiscovery is a in-process service discovery.
// Clients and servers are in one process and communicate without tcp/udp.
type InpreocessDiscovery struct {
}

// NewInpreocessDiscovery returns a new InpreocessDiscovery.
func NewInpreocessDiscovery() ServiceDiscovery {
	return &InpreocessDiscovery{}
}

// GetServices returns the static server
func (d InpreocessDiscovery) GetServices() []*KVPair {
	return []*KVPair{&KVPair{Key: "inprocess@127.0.0.1:0", Value: ""}}
}

// WatchService returns a nil chan.
func (d InpreocessDiscovery) WatchService() chan []*KVPair {
	return nil
}
