package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/smallnest/rpcx/v5/log"
	"github.com/smallnest/rpcx/v5/util"
)

// NacosDiscovery is a nacos service discovery.
// It always returns the registered servers in nacos.
type NacosDiscovery struct {
	servicePath string
	// nacos client config
	ClientConfig constant.ClientConfig
	// nacos server config
	ServerConfig []constant.ServerConfig
	Cluster      string

	namingClient naming_client.INamingClient

	pairs []*KVPair
	chans []chan []*KVPair
	mu    sync.Mutex

	filter                  ServiceDiscoveryFilter
	RetriesAfterWatchFailed int

	stopCh chan struct{}
}

// NewNacosDiscovery returns a new NacosDiscovery.
func NewNacosDiscovery(servicePath string, cluster string, clientConfig constant.ClientConfig, serverConfig []constant.ServerConfig) ServiceDiscovery {
	d := &NacosDiscovery{
		servicePath:  servicePath,
		Cluster:      cluster,
		ClientConfig: clientConfig,
		ServerConfig: serverConfig,
	}

	namingClient, err := clients.CreateNamingClient(map[string]interface{}{
		"clientConfig":  clientConfig,
		"serverConfigs": serverConfig,
	})
	if err != nil {
		log.Errorf("failed to create NacosDiscovery: %v", err)
		return nil
	}

	d.namingClient = namingClient

	d.fetch()
	go d.watch()
	return d
}

func NewNacosDiscoveryWithClient(servicePath string, cluster string, namingClient naming_client.INamingClient) ServiceDiscovery {
	d := &NacosDiscovery{
		servicePath: servicePath,
		Cluster:     cluster,
	}

	d.namingClient = namingClient

	d.fetch()
	go d.watch()
	return d
}

func (d *NacosDiscovery) fetch() {
	service, err := d.namingClient.GetService(vo.GetServiceParam{
		ServiceName: d.servicePath,
		Clusters:    []string{d.Cluster},
	})

	if err != nil {
		log.Errorf("failed to get service %s: %v", d.servicePath, err)
		return
	}
	var pairs = make([]*KVPair, 0, len(service.Hosts))
	for _, inst := range service.Hosts {
		network := inst.Metadata["network"]
		ip := inst.Ip
		port := inst.Port
		key := fmt.Sprintf("%s@%s:%d", network, ip, port)
		pair := &KVPair{Key: key, Value: util.ConvertMap2String(inst.Metadata)}
		if d.filter != nil && !d.filter(pair) {
			continue
		}
		pairs = append(pairs, pair)
	}

	d.pairs = pairs
}

// NewNacosDiscoveryTemplate returns a new NacosDiscovery template.
func NewNacosDiscoveryTemplate(cluster string, clientConfig constant.ClientConfig, serverConfig []constant.ServerConfig) ServiceDiscovery {
	return &NacosDiscovery{
		Cluster:      cluster,
		ClientConfig: clientConfig,
		ServerConfig: serverConfig,
	}
}

// Clone clones this ServiceDiscovery with new servicePath.
func (d *NacosDiscovery) Clone(servicePath string) ServiceDiscovery {
	return NewNacosDiscovery(servicePath, d.Cluster, d.ClientConfig, d.ServerConfig)
}

// SetFilter sets the filer.
func (d *NacosDiscovery) SetFilter(filter ServiceDiscoveryFilter) {
	d.filter = filter
}

// GetServices returns the servers
func (d *NacosDiscovery) GetServices() []*KVPair {
	return d.pairs
}

// WatchService returns a nil chan.
func (d *NacosDiscovery) WatchService() chan []*KVPair {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch := make(chan []*KVPair, 10)
	d.chans = append(d.chans, ch)
	return ch
}

func (d *NacosDiscovery) RemoveWatcher(ch chan []*KVPair) {
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

func (d *NacosDiscovery) watch() {
	param := &vo.SubscribeParam{
		ServiceName: d.servicePath,
		Clusters:    []string{d.Cluster},
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			var pairs = make([]*KVPair, 0, len(services))
			for _, inst := range services {
				network := inst.Metadata["network"]
				ip := inst.Ip
				port := inst.Port
				key := fmt.Sprintf("%s@%s:%d", network, ip, port)
				pair := &KVPair{Key: key, Value: util.ConvertMap2String(inst.Metadata)}
				if d.filter != nil && !d.filter(pair) {
					continue
				}
				pairs = append(pairs, pair)
			}
			d.pairs = pairs

			d.mu.Lock()
			for _, ch := range d.chans {
				ch := ch
				go func() {
					defer func() {
						recover()
					}()
					select {
					case ch <- d.pairs:
					case <-time.After(time.Minute):
						log.Warn("chan is full and new change has been dropped")
					}
				}()
			}
			d.mu.Unlock()
		},
	}

	err := d.namingClient.Subscribe(param)

	// if failed to Subscribe, retry
	if err != nil {
		var tempDelay time.Duration
		retry := d.RetriesAfterWatchFailed
		for d.RetriesAfterWatchFailed < 0 || retry >= 0 {
			err := d.namingClient.Subscribe(param)
			if err != nil {
				if d.RetriesAfterWatchFailed > 0 {
					retry--
				}
				if tempDelay == 0 {
					tempDelay = 1 * time.Second
				} else {
					tempDelay *= 2
				}
				if max := 30 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Warnf("can not subscribe (with retry %d, sleep %v): %s: %v", retry, tempDelay, d.servicePath, err)
				time.Sleep(tempDelay)
				continue
			}
			break
		}
	}

}

func (d *NacosDiscovery) Close() {
	close(d.stopCh)
}
