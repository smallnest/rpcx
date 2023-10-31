package client

import (
	"encoding/json"
	"os"
)

// ServiceDiscoveryFilter can be used to filter services with customized logics.
// Servers can register its services but clients can use the customized filter to select some services.
// It returns true if ServiceDiscovery wants to use this service, otherwise it returns false.
type ServiceDiscoveryFilter func(kvp *KVPair) bool

// ServiceDiscovery defines ServiceDiscovery of zookeeper, etcd and consul
type ServiceDiscovery interface {
	GetServices() []*KVPair       // return all services in the registry
	WatchService() chan []*KVPair // watch the change of services, it's a golang channel
	RemoveWatcher(ch chan []*KVPair)
	Clone(servicePath string) (ServiceDiscovery, error)
	SetFilter(ServiceDiscoveryFilter) // set customized filter to filter services
	Close()
}

type cachedServiceDiscovery struct {
	threshold  int
	cachedFile string
	cached     []*KVPair

	ServiceDiscovery
}

// CacheDiscovery caches the services in a file, it will return the cached services if the number of services is greater than threshold.
// It is very useful when the register center is lost.
func CacheDiscovery(threshold int, cachedFile string, discovery ServiceDiscovery) ServiceDiscovery {
	if cachedFile == "" {
		cachedFile = ".cache/discovery.json"
	}

	return &cachedServiceDiscovery{
		threshold:        threshold,
		cachedFile:       cachedFile,
		ServiceDiscovery: discovery,
	}
}

func (cd *cachedServiceDiscovery) GetServices() []*KVPair {
	kvPairs := cd.ServiceDiscovery.GetServices()

	n := len(kvPairs)
	if n > cd.threshold {
		if n > len(cd.cached) { // strictly we should compare the content of the cached file, but only compare the length for performance
			cd.cached = kvPairs
			cd.storeCached(kvPairs)
		}

		return kvPairs
	}

	if len(cd.cached) == 0 {
		cd.loadCached()
	}

	return cd.cached
}

func (cd *cachedServiceDiscovery) storeCached(kvPairs []*KVPair) {
	data, _ := json.Marshal(kvPairs)
	os.WriteFile(cd.cachedFile, data, 0644)
}

func (cd *cachedServiceDiscovery) loadCached() (kvPairs []*KVPair) {
	data, err := os.ReadFile(cd.cachedFile)
	if err != nil || len(data) == 0 {
		return
	}

	json.Unmarshal(data, &kvPairs)

	return kvPairs
}
