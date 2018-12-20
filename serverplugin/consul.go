// +build consul

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
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/log"
)

func init() {
	consul.Register()
}

// ConsulRegisterPlugin implements consul registry.
type ConsulRegisterPlugin struct {
	// service address, for example, tcp@127.0.0.1:8972, quic@127.0.0.1:1234
	ServiceAddress string
	// consul addresses
	ConsulServers []string
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

	dying chan struct{}
	done  chan struct{}
}

// Start starts to connect consul cluster
func (p *ConsulRegisterPlugin) Start() error {
	if p.done == nil {
		p.done = make(chan struct{})
	}
	if p.dying == nil {
		p.dying = make(chan struct{})
	}

	if p.kv == nil {
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, p.Options)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}

	err := p.kv.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", p.BasePath, err)
		return err
	}

	if p.UpdateInterval > 0 {
		ticker := time.NewTicker(p.UpdateInterval)
		go func() {
			defer p.kv.Close()

			// refresh service TTL
			for {
				select {
				case <-p.dying:
					close(p.done)
					return
				case <-ticker.C:
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
							log.Warnf("can't get data of node: %s, will re-create, because of %v", nodePath, err.Error())

							p.metasLock.RLock()
							meta := p.metas[name]
							p.metasLock.RUnlock()

							err = p.kv.Put(nodePath, []byte(meta), &store.WriteOptions{TTL: p.UpdateInterval * 3})
							if err != nil {
								log.Errorf("cannot re-create consul path %s: %v", nodePath, err)
							}
						} else {
							v, _ := url.ParseQuery(string(kvPaire.Value))
							v.Set("tps", string(data))
							p.kv.Put(nodePath, []byte(v.Encode()), &store.WriteOptions{TTL: p.UpdateInterval * 3})
						}
					}
				}
			}
		}()
	}

	return nil
}

// Stop unregister all services.
func (p *ConsulRegisterPlugin) Stop() error {
	close(p.dying)
	<-p.done

	if p.kv == nil {
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, p.Options)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}

	for _, name := range p.Services {
		nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
		exist, err := p.kv.Exists(nodePath)
		if err != nil {
			log.Errorf("cannot delete path %s: %v", nodePath, err)
			continue
		}
		if exist {
			p.kv.Delete(nodePath)
			log.Infof("delete path %s", nodePath, err)
		}
	}
	return nil
}

// HandleConnAccept handles connections from clients
func (p *ConsulRegisterPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	if p.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
		clientMeter.Mark(1)
	}
	return conn, true
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *ConsulRegisterPlugin) Register(name string, rcvr interface{}, metadata string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("Register service `name` can't be empty")
		return
	}

	if p.kv == nil {
		consul.Register()
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, nil)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}
	err = p.kv.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", p.BasePath, err)
		return err
	}

	nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)
	err = p.kv.Put(nodePath, []byte(name), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
		return err
	}

	nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
	err = p.kv.Put(nodePath, []byte(metadata), &store.WriteOptions{TTL: p.UpdateInterval * 2})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
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

func (p *ConsulRegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("Unregister service `name` can't be empty")
		return
	}

	if p.kv == nil {
		consul.Register()
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, nil)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.kv = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}
	err = p.kv.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", p.BasePath, err)
		return err
	}

	nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)

	err = p.kv.Put(nodePath, []byte(name), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
		return err
	}

	nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

	err = p.kv.Delete(nodePath)
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
		return err
	}

	var services = make([]string, 0, len(p.Services)-1)
	for _, s := range p.Services {
		if s != name {
			services = append(services, s)
		}
	}
	p.Services = services

	p.metasLock.Lock()
	if p.metas == nil {
		p.metas = make(map[string]string)
	}
	delete(p.metas, name)
	p.metasLock.Unlock()
	return
}
