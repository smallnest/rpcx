package plugin

import (
	"log"
	"strconv"
	"errors"
	"golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	"github.com/rcrowley/go-metrics"
	"time"
	"fmt"
	"net/url"
	"net"
	"strings"
)

//EtcdRegisterPlugin a register plugin which can register services into etcd for cluster
type EtcdV3RegisterPlugin struct {
	ServiceAddress 		string
	EtcdServers    		[]string
	BasePath       		string
	Metrics        		metrics.Registry
	Services       		[]string
	UpdateInterval 		time.Duration
	UpdateIntervalNum 	int64
	KeysAPI        		*clientv3.Client
	ticker         		*time.Ticker
	DialTimeout    		time.Duration
	Username 		string
	Password 		string
}

// Start starts to connect etcd v3 cluster
func (this *EtcdV3RegisterPlugin) Start() (err error) {
	var (
		resp	*clientv3.GetResponse
		v        url.Values
		nodePath string
		ttl 	*clientv3.LeaseGrantResponse
	)

	cli,err := clientv3.New(clientv3.Config{
		Endpoints:		this.EtcdServers,
		DialTimeout: 		this.DialTimeout,
		Username:		this.Username,
		Password: 		this.Password,
	})

	if err != nil {
		log.Println("new client: "+err.Error())
		return
	}
	this.KeysAPI = cli

	if this.UpdateInterval > 0 {
		this.ticker = time.NewTicker(this.UpdateInterval)
		go func() {
			for range this.ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", this.Metrics)
				data := strconv.FormatInt(clientMeter.Count()/60, 10)
				//set this same metrics for all services at this server

				for _,name := range this.Services {
					nodePath = fmt.Sprintf("%s/%s/%s", this.BasePath, name, this.ServiceAddress)

					ctx,cancel := context.WithTimeout(context.Background(),this.DialTimeout)
					resp,err = this.KeysAPI.Get(ctx,nodePath)
					defer cancel()
					if err != nil {
						log.Println("get etcd key failed. " + err.Error())
					} else {
						if v,err = url.ParseQuery(string(resp.Kvs[0].Value)); err != nil {
							continue
						}
						v.Set("tps",string(data))

						//TTL
						ttl,err = cli.Grant(context.TODO(),this.UpdateIntervalNum+5)
						if err != nil {
							log.Println("V3 TTL Grant:"+err.Error())
						}
						//KeepAlive TTL alive forever
						ch,kaerr := this.KeysAPI.KeepAlive(context.TODO(),ttl.ID)
						if kaerr != nil {
							log.Printf("Set ttl Keepalive is forver:%s",kaerr.Error())
						}
						select {
						case ka:=<-ch:
							log.Printf("TTL value is %d",ka.TTL)
						}
						_,err = this.KeysAPI.Put(context.TODO(),nodePath,v.Encode(),clientv3.WithLease(ttl.ID))
						if err != nil {
							log.Printf("Put key %s value %s :%s",nodePath,v.Encode(),err.Error())
						}
					}

				}
			}
		}()
	}
	return
}

// HandleConnAccept handles connections from clients
func (this *EtcdV3RegisterPlugin) HandleConnAccept(net.Conn) bool {
	if this.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", this.Metrics)
		clientMeter.Mark(1)
	}
	return true
}

//Close closes this plugin
func (this *EtcdV3RegisterPlugin) Close() {
	this.ticker.Stop()
}

//V3 KV with Put
func (this *EtcdV3RegisterPlugin) Put(key,value string,opts ...clientv3.OpOption ) error {
	_,err := this.KeysAPI.Put(context.TODO(),key,value,opts...)
	if err != nil {
		log.Printf("Put %s %s error %s",key,value,err.Error())
		return err
	}
	return nil
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (this *EtcdV3RegisterPlugin) Register(name string, metadata ...string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("service `name` can't be empty!")
		return
	}
	//nodePath := fmt.Sprintf("%s/%s", this.BasePath, name)
	//if err = this.Put(nodePath,"dir"); err != nil {
	//	log.Fatal(err.Error())
	//	return
	//}
	nodePath := fmt.Sprintf("%s/%s/%s", this.BasePath, name, this.ServiceAddress)

	err = this.Put(nodePath,strings.Join(metadata,"&"))
	if err != nil {
		return
	}

	if !IsContainsV3(this.Services,fmt.Sprintf("%s/%s", this.BasePath, name)) {
		this.Services = append(this.Services,fmt.Sprintf("%s/%s", this.BasePath, name))
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
func (this *EtcdV3RegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("unregister service `name` cann't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s/%s", this.BasePath, name, this.ServiceAddress)

	_,err = this.KeysAPI.Delete(context.TODO(),nodePath,clientv3.WithPrefix())
	if err != nil {
		return
	}
	// because plugin.Start() method will be executed by timer continuously
	// so it need to remove the service name from service list
	if this.Services == nil || len(this.Services) <= 0 {
		return nil
	}
	var index int = 0
	for index = 0; index < len(this.Services); index++ {
		if this.Services[index] == fmt.Sprintf("%s/%s", this.BasePath, name) {
			break
		}
	}
	if index != len(this.Services) {
		this.Services = append(this.Services[:index], this.Services[index+1:]...)
	}

	return
}

// Name return name of this plugin.
func (plugin *EtcdV3RegisterPlugin) Name() string {
	return "EtcdRegisterPlugin"
}
