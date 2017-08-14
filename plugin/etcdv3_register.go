package plugin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/log"
	"golang.org/x/net/context"
)

//EtcdV3RegisterPlugin a register plugin which can register services into etcd for cluster
type EtcdV3RegisterPlugin struct {
	ServiceAddress      string
	EtcdServers         []string
	BasePath            string
	Metrics             metrics.Registry
	Services            []string
	ttls                map[string]*clientv3.LeaseGrantResponse
	ttlsLock            sync.Mutex
	UpdateIntervalInSec int64
	KeysAPI             *clientv3.Client
	ticker              *time.Ticker
	DialTimeout         time.Duration
	Username            string
	Password            string
}

// Start starts to connect etcd v3 cluster
func (p *EtcdV3RegisterPlugin) Start() (err error) {
	if p.ttls == nil {
		p.ttls = make(map[string]*clientv3.LeaseGrantResponse)
	}
	var (
		resp     *clientv3.GetResponse
		v        url.Values
		nodePath string
		ttl      *clientv3.LeaseGrantResponse
	)

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   p.EtcdServers,
		DialTimeout: p.DialTimeout,
		Username:    p.Username,
		Password:    p.Password,
	})

	if err != nil {
		log.Infof("new client: %v", err.Error())
		return
	}
	p.KeysAPI = cli

	if p.UpdateIntervalInSec > 0 {
		p.ticker = time.NewTicker(time.Duration(p.UpdateIntervalInSec*2) * time.Second)
		go func() {
			for range p.ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
				data := strconv.FormatInt(clientMeter.Count()/60, 10)
				//set this same metrics for all services at this server

				for _, name := range p.Services {
					nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)
					if p.DialTimeout > 0 {
						ctx, _ := context.WithTimeout(context.Background(), p.DialTimeout)
						resp, err = p.KeysAPI.Get(ctx, nodePath)
					} else {
						resp, err = p.KeysAPI.Get(context.TODO(), nodePath)
					}
					if err != nil {
						log.Infof("get etcd key failed: %v", err.Error())
					} else {
						if len(resp.Kvs) == 0 { // this service has not been registered into etcd
							continue
						}
						if v, err = url.ParseQuery(string(resp.Kvs[0].Value)); err != nil {
							continue
						}
						v.Set("tps", string(data))

						// add ttl and keepalive
						p.ttlsLock.Lock()
						ttl = p.ttls[nodePath]
						if ttl == nil {
							ttl, err = cli.Grant(context.TODO(), p.UpdateIntervalInSec)
							if err != nil {
								log.Infof("V3 TTL Grant: %v", err.Error())
								p.ttlsLock.Unlock()
								continue
							}

							//KeepAlive TTL alive forever
							ch, kaerr := p.KeysAPI.KeepAlive(context.TODO(), ttl.ID)
							if kaerr != nil {
								log.Infof("Set ttl Keepalive is forver: %s", kaerr.Error())
								p.ttlsLock.Unlock()
								continue
							}

							ka := <-ch
							log.Debugf("TTL value is %d", ka.TTL)
							p.ttls[nodePath] = ttl
						}
						p.ttlsLock.Unlock()

						_, err = p.KeysAPI.Put(context.TODO(), nodePath, v.Encode(), clientv3.WithLease(ttl.ID))
						if err != nil {
							log.Infof("Put key %s value %s : %s", nodePath, v.Encode(), err.Error())
						}
					}

				}
			}
		}()
	}
	return
}

// HandleConnAccept handles connections from clients
func (p *EtcdV3RegisterPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	if p.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
		clientMeter.Mark(1)
	}
	return conn, true
}

//Close closes this plugin
func (p *EtcdV3RegisterPlugin) Close() {
	p.ticker.Stop()
}

//Put KV by V3 API
func (p *EtcdV3RegisterPlugin) Put(key, value string, opts ...clientv3.OpOption) error {
	_, err := p.KeysAPI.Put(context.TODO(), key, value, opts...)
	if err != nil {
		log.Infof("Put %s %s error %s", key, value, err.Error())
		return err
	}
	return nil
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *EtcdV3RegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("service `name` can't be empty!")
		return
	}
	//nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)
	//if err = p.Put(nodePath,"dir"); err != nil {
	//	log.Fatal(err.Error())
	//	return
	//}
	nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

	err = p.Put(nodePath, strings.Join(metadata, "&"))
	if err != nil {
		log.Infof("failed to put service meta: %+v", err)
		return
	}

	if !IsContainsV3(p.Services, name) {
		p.Services = append(p.Services, name)
	}

	// add ttl and keepalive
	p.ttlsLock.Lock()
	ttl := p.ttls[nodePath]
	if ttl == nil {
		ttl, err = p.KeysAPI.Grant(context.TODO(), p.UpdateIntervalInSec)
		if err != nil {
			log.Infof("V3 TTL Grant: %v", err.Error())
			p.ttlsLock.Unlock()
			return err
		}

		//KeepAlive TTL alive forever
		ch, kaerr := p.KeysAPI.KeepAlive(context.TODO(), ttl.ID)
		if kaerr != nil {
			log.Infof("Set ttl Keepalive is forver: %s", kaerr.Error())
			p.ttlsLock.Unlock()
			return kaerr
		}

		ka := <-ch
		log.Debugf("TTL value is %d", ka.TTL)
		p.ttls[nodePath] = ttl
	}
	p.ttlsLock.Unlock()

	_, err = p.KeysAPI.Put(context.TODO(), nodePath, strings.Join(metadata, "&"), clientv3.WithLease(ttl.ID))
	if err != nil {
		log.Infof("Put key %s value %s : %s", nodePath, strings.Join(metadata, "&"), err.Error())
	}

	return
}

func IsContainsV3(list []string, element string) (exist bool) {
	exist = false
	if list == nil || len(list) <= 0 {
		return
	}
	for index := 0; index < len(list); index++ {
		if list[index] == element {
			return true
		}
	}
	return
}

// Unregister a service from etcd but this service still exists in this node.
func (p *EtcdV3RegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("unregister service `name` cann't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

	p.ttlsLock.Lock()
	ttl := p.ttls[nodePath]
	if ttl != nil {
		p.KeysAPI.Revoke(context.TODO(), ttl.ID)
	}
	p.ttlsLock.Unlock()

	_, err = p.KeysAPI.Delete(context.TODO(), nodePath, clientv3.WithPrefix())
	if err != nil {
		return
	}
	// because plugin.Start() method will be executed by timer continuously
	// so it need to remove the service name from service list
	if p.Services == nil || len(p.Services) <= 0 {
		return nil
	}
	var index int
	for index = 0; index < len(p.Services); index++ {
		if p.Services[index] == name {
			break
		}
	}
	if index != len(p.Services) {
		p.Services = append(p.Services[:index], p.Services[index+1:]...)
	}

	return
}

// Name returns name of this plugin.
func (p *EtcdV3RegisterPlugin) Name() string {
	return "EtcdV3RegisterPlugin"
}
