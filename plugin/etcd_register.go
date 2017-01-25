package plugin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/log"
)

//EtcdRegisterPlugin a register plugin which can register services into etcd for cluster
type EtcdRegisterPlugin struct {
	ServiceAddress string
	EtcdServers    []string
	BasePath       string
	Metrics        metrics.Registry
	Services       []string
	UpdateInterval time.Duration
	KeysAPI        client.KeysAPI
	ticker         *time.Ticker
}

// Start starts to connect etcd cluster
func (p *EtcdRegisterPlugin) Start() (err error) {
	var (
		resp     *client.Response
		v        url.Values
		nodePath string
	)
	cli, err := client.New(client.Config{
		Endpoints:               p.EtcdServers,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 5 * time.Second,
	})

	if err != nil {
		log.Infof("new client: %v", err.Error())
		return
	}
	p.KeysAPI = client.NewKeysAPI(cli)
	if err = p.forceMkdirs(p.BasePath); err != nil {
		log.Infof("can't make dirs: %s because of %s", p.BasePath, err.Error())
		return
	}

	if p.UpdateInterval > 0 {
		p.ticker = time.NewTicker(p.UpdateInterval)
		go func() {
			for range p.ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
				data := strconv.FormatInt(clientMeter.Count()/60, 10)
				//set this same metrics for all services at this server

				for _, name := range p.Services {
					// if err = p.mkdirs(fmt.Sprintf("%s/%s", p.BasePath, name)); err != nil {
					// 	log.Infof(err.Error())
					// 	continue
					// }

					nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

					resp, err = p.KeysAPI.Get(context.TODO(), nodePath, &client.GetOptions{
						Recursive: false,
					})
					if err != nil {
						log.Infof("get etcd key failed: %v", err.Error())
					} else {
						if v, err = url.ParseQuery(resp.Node.Value); err != nil {
							continue
						}
						v.Set("tps", string(data))

						_, err = p.KeysAPI.Set(context.TODO(), nodePath, v.Encode(), &client.SetOptions{
							PrevExist: client.PrevIgnore,
							TTL:       p.UpdateInterval + 10*time.Second,
						})

						if err != nil {
							log.Infof("set etcd key failed: %v", err.Error())
						}
					}

				}

			}
		}()
	}

	return
}

// HandleConnAccept handles connections from clients
func (p *EtcdRegisterPlugin) HandleConnAccept(net.Conn) bool {
	if p.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
		clientMeter.Mark(1)
	}
	return true
}

//Close closes this plugin
func (p *EtcdRegisterPlugin) Close() {
	p.ticker.Stop()
}

// func (p *EtcdRegisterPlugin) mkdirs(path string) (err error) {
// 	if "" == strings.TrimSpace(path) {
// 		err = errors.New("etcd dir `path` can't be empty!")
// 		return
// 	}
// 	_, err = p.KeysAPI.Set(context.TODO(), path, "",
// 		&client.SetOptions{
// 			Dir:       true,
// 			PrevExist: client.PrevNoExist,
// 		})

// 	return
// }

func (p *EtcdRegisterPlugin) forceMkdirs(path string) (err error) {
	if "" == strings.TrimSpace(path) {
		err = errors.New("etcd dir `path` can't be empty!")
		return
	}
	_, err = p.KeysAPI.Set(context.TODO(), path, "",
		&client.SetOptions{
			PrevExist: client.PrevNoExist,
			Dir:       true,
		})

	if err != nil {
		if e, ok := err.(client.Error); ok && e.Code == client.ErrorCodeNodeExist {
			err = nil
		}
	}
	return
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *EtcdRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("service `name` can't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s", p.BasePath, name)
	if err = p.forceMkdirs(nodePath); err != nil {
		log.Fatal(err.Error())
		return
	}

	nodePath = fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

	_, err = p.KeysAPI.Set(context.TODO(), nodePath, strings.Join(metadata, "&"),
		&client.SetOptions{
			PrevExist: client.PrevIgnore,
			TTL:       p.UpdateInterval + 10*time.Second,
		})

	if err != nil {
		return
	}

	if !IsContains(p.Services, name) {
		p.Services = append(p.Services, name)
	}
	return
}

func IsContains(list []string, element string) (exist bool) {
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
func (p *EtcdRegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("unregister service `name` cann't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s/%s", p.BasePath, name, p.ServiceAddress)

	_, err = p.KeysAPI.Delete(context.TODO(), nodePath, &client.DeleteOptions{Recursive: true})
	if err != nil {
		return
	}
	// because p.Start() method will be executed by timer continuously
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

// Name return name of this p.
func (p *EtcdRegisterPlugin) Name() string {
	return "EtcdRegisterPlugin"
}
