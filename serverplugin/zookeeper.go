// +build zookeeper

package serverplugin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store/zookeeper"

	"github.com/docker/libkv/store"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/log"
)

func init() {
	zookeeper.Register()
}

// ZooKeeperRegisterPlugin implements zookeeper registry.
type ZooKeeperRegisterPlugin struct {
	// service address, for example, tcp@127.0.0.1:8972, quic@127.0.0.1:1234
	ServiceAddress string
	// zookeeper addresses
	ZooKeeperServers []string
	// base path for rpcx server, for example com/example/rpcx
	BasePath string
	Metrics  metrics.Registry
	// Registered services
	Services       []string
	metasLock      sync.RWMutex
	metas          map[string]string
	UpdateInterval time.Duration

	Options *store.Config
	kv      store.Store
}

// Start starts to connect zookeeper cluster
func (p *ZooKeeperRegisterPlugin) Start() error {
	if p.kv == nil {
		kv, err := libkv.NewStore(store.ZK, p.ZooKeeperServers, p.Options)
		if err != nil {
			log.Errorf("cannot create zk registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}

	err := p.kv.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create zk path %s: %v", p.BasePath, err)
		return err
	}

	if p.UpdateInterval > 0 {
		ticker := time.NewTicker(p.UpdateInterval)
		go func() {
			defer p.kv.Close()

			// refresh service TTL
			for range ticker.C {
				var data []byte
				if p.Metrics != nil {
					clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
					data = []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
				}
				//set this same metrics for all services at this server
				for _, name := range p.Services {
					nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
					kvPaire, err := p.kv.Get(nodePath)
					if err != nil {
						log.Infof("can't get data of node: %s, because of %v", nodePath, err.Error())

						p.metasLock.RLock()
						meta := p.metas[name]
						p.metasLock.RUnlock()

						err = p.kv.Put(nodePath, []byte(meta), &store.WriteOptions{TTL: p.UpdateInterval * 3})
						if err != nil {
							log.Errorf("cannot re-create zookeeper path %s: %v", nodePath, err)
						}
					} else {
						v, _ := url.ParseQuery(string(kvPaire.Value))
						v.Set("tps", string(data))
						p.kv.Put(nodePath, []byte(v.Encode()), &store.WriteOptions{TTL: p.UpdateInterval * 3})
					}
				}

			}
		}()
	}

	return nil
}

// HandleConnAccept handles connections from clients
func (p *ZooKeeperRegisterPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	if p.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
		clientMeter.Mark(1)
	}
	return conn, true
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *ZooKeeperRegisterPlugin) Register(name string, rcvr interface{}, metadata string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("Register service `name` can't be empty")
		return
	}

	if p.kv == nil {
		zookeeper.Register()
		kv, err := libkv.NewStore(store.ZK, p.ZooKeeperServers, nil)
		if err != nil {
			log.Errorf("cannot create zk registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}
	err = p.kv.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create zk path %s: %v", p.BasePath, err)
		return err
	}

	nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)
	err = p.kv.Put(nodePath, []byte(name), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create zk path %s: %v", nodePath, err)
		return err
	}

	nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
	err = p.kv.Put(nodePath, []byte(metadata), &store.WriteOptions{TTL: p.UpdateInterval * 2})
	if err != nil {
		log.Errorf("cannot create zk path %s: %v", nodePath, err)
		return err
	}

	p.Services = append(p.Services, name)

	p.metasLock.Lock()
	if p.metas == nil {
		p.metas = make(map[string]string)
	}
	p.metas[name] = metadata
	p.metasLock.Unlock()
	return
}
