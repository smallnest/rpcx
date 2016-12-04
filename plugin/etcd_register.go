package plugin

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/rcrowley/go-metrics"
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
func (plugin *EtcdRegisterPlugin) Start() (err error) {
	cli, err := client.New(client.Config{
		Endpoints:               plugin.EtcdServers,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 5 * time.Second,
	})

	if err != nil {
		return err
	}
	plugin.KeysAPI = client.NewKeysAPI(cli)
	plugin.mkdirs(plugin.BasePath)

	if plugin.UpdateInterval > 0 {
		plugin.ticker = time.NewTicker(plugin.UpdateInterval)
		go func() {
			for range plugin.ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Metrics)
				data := strconv.FormatInt(clientMeter.Count()/60, 10)
				//set this same metrics for all services at this server

				for _, name := range plugin.Services {
					plugin.mkdirs(fmt.Sprintf("%s/%s", plugin.BasePath, name))

					nodePath := fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)

					resp, err := plugin.KeysAPI.Get(context.TODO(), nodePath, &client.GetOptions{
						Recursive: false,
					})
					if err == nil {
						v, _ := url.ParseQuery(resp.Node.Value)
						v.Set("tps", string(data))

						_, err = plugin.KeysAPI.Set(context.TODO(), nodePath, v.Encode(), &client.SetOptions{
							PrevExist: client.PrevIgnore,
							TTL:       plugin.UpdateInterval + 10*time.Second,
						})

						if err != nil {
							log.Fatal(err)
						}
					}

				}

			}
		}()
	}

	return
}

// HandleConnAccept handles connections from clients
func (plugin *EtcdRegisterPlugin) HandleConnAccept(net.Conn) bool {
	if plugin.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Metrics)
		clientMeter.Mark(1)
	}
	return true
}

//Close closes this plugin
func (plugin *EtcdRegisterPlugin) Close() {
	plugin.ticker.Stop()
}

func (plugin *EtcdRegisterPlugin) mkdirs(path string) (err error) {
	if "" == strings.TrimSpace(path) {
		err = errors.New("etcd dir `path` can't be empty!")
		return
	}
	_, err = plugin.KeysAPI.Set(context.TODO(), path, "",
		&client.SetOptions{
			Dir:       true,
			PrevExist: client.PrevNoExist,
		})

	return
}

func (plugin *EtcdRegisterPlugin) forceMkdirs(path string) (err error) {
	if "" == strings.TrimSpace(path) {
		err = errors.New("etcd dir `path` can't be empty!")
		return
	}
	_, err = plugin.KeysAPI.Set(context.TODO(), path, "",
		&client.SetOptions{
			PrevExist: client.PrevIgnore,
			Dir:       true,
		})

	return
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (plugin *EtcdRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("service `name` can't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s", plugin.BasePath, name)
	if err = plugin.mkdirs(nodePath); err != nil {
		return
	}

	nodePath = fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)

	_, err = plugin.KeysAPI.Set(context.TODO(), nodePath, strings.Join(metadata, "&"),
		&client.SetOptions{
			PrevExist: client.PrevIgnore,
			TTL:       plugin.UpdateInterval + 10*time.Second,
		})

	if err != nil {
		return
	}

	if !IsContains(plugin.Services, name) {
		plugin.Services = append(plugin.Services, name)
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
func (plugin *EtcdRegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("unregister service `name` cann't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)

	_, err = plugin.KeysAPI.Delete(context.TODO(), nodePath, &client.DeleteOptions{Recursive: true})
	if err != nil {
		return
	}
	return
}

// Name return name of this plugin.
func (plugin *EtcdRegisterPlugin) Name() string {
	return "EtcdRegisterPlugin"
}
