// +build consul

package serverplugin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
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
	UpdateInterval time.Duration

	Options *store.Config
	KV      store.Store
}

// Start starts to connect consul cluster
func (p *ConsulRegisterPlugin) Start() error {
	if p.KV == nil {
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, p.Options)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.KV = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}

	err := p.KV.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", p.BasePath, err)
		return err
	}

	if p.UpdateInterval > 0 {
		ticker := time.NewTicker(p.UpdateInterval)
		go func() {
			defer p.KV.Close()

			// refresh service TTL
			for range ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
				data := []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
				//set this same metrics for all services at this server
				for _, name := range p.Services {
					nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
					kvPaire, err := p.KV.Get(nodePath)
					if err != nil {
						log.Infof("can't get data of node: %s, because of %v", nodePath, err.Error())
					} else {
						v, _ := url.ParseQuery(string(kvPaire.Value))
						v.Set("tps", string(data))
						p.KV.Put(nodePath, []byte(v.Encode()), &store.WriteOptions{TTL: p.UpdateInterval * 2})
					}
				}

			}
		}()
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

	if p.KV == nil {
		consul.Register()
		kv, err := libkv.NewStore(store.CONSUL, p.ConsulServers, nil)
		if err != nil {
			log.Errorf("cannot create consul registry: %v", err)
			return err
		}
		p.KV = kv
	}

	if p.BasePath[0] == '/' {
		p.BasePath = p.BasePath[1:]
	}
	err = p.KV.Put(p.BasePath, []byte("rpcx_path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", p.BasePath, err)
		return err
	}

	nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)
	err = p.KV.Put(nodePath, []byte(name), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
		return err
	}

	nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
	err = p.KV.Put(nodePath, []byte(p.ServiceAddress), &store.WriteOptions{TTL: p.UpdateInterval * 2})
	if err != nil {
		log.Errorf("cannot create consul path %s: %v", nodePath, err)
		return err
	}

	p.Services = append(p.Services, name)
	return
}
