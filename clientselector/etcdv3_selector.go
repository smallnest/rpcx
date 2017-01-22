package clientselector

import (
	"log"
	"time"
	"sync"
	"net/rpc"
	"math/rand"
	"github.com/lflxp/rpcx"
	"golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	mvccpb "github.com/coreos/etcd/mvcc/mvccpb"
	"strings"
	"net"
	"net/url"
	"strconv"
	"fmt"
	"errors"
)

// EtcdClientSelector is used to select a rpc server from etcd.
type EtcdV3ClientSelector struct {
	EtcdServers        	[]string
	KeysAPI            	*clientv3.Client
	ticker             	*time.Ticker
	sessionTimeout     	time.Duration
	BasePath           	string //should endwith serviceName
	Servers            	[]string
	Group              	string
	clientAndServer    	map[string]*rpc.Client
	clientRWMutex      	sync.RWMutex
	metadata          	 map[string]string
	Latitude          	 float64
	Longitude       	   float64
	WeightedServers   	 []*Weighted
	SelectMode       	  rpcx.SelectMode
	dailTimeout      	  time.Duration
	rnd               	 *rand.Rand
	currentServer    	  int
	len              	  int
	HashServiceAndArgs 	HashServiceAndArgs
	Client            	 *rpcx.Client
	DialTimeout    		time.Duration
	Username 		string
	Password 		string
	UpdateIntervalNum 	int64
}

//SetClient set a Client in order that clientSelector can uses it
func (this *EtcdV3ClientSelector) SetClient(c *rpcx.Client) {
	this.Client = c
}

//SetSelectMode sets SelectMode
func (this *EtcdV3ClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	this.SelectMode = sm
}

//AllClients returns rpc.Clients to all servers
func (this *EtcdV3ClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range this.Servers {
		ss := strings.Split(sv, "@")
		c, err := rpcx.NewDirectRPCClient(this.Client, clientCodecFunc, ss[0], ss[1], this.dailTimeout)
		if err != nil {
			log.Fatal("rpc client connect server failed. " + err.Error())
			continue
		} else {
			clients = append(clients, c)
		}
	}

	return clients
}

// NewEtcdClientSelector creates a EtcdClientSelector
func NewEtcdV3ClientSelector(etcdServers []string, basePath string, sessionTimeout time.Duration, sm rpcx.SelectMode, dailTimeout time.Duration) *EtcdV3ClientSelector {
	selector := &EtcdV3ClientSelector{
		EtcdServers:     etcdServers,
		BasePath:        basePath,
		sessionTimeout:  sessionTimeout,
		SelectMode:      sm,
		dailTimeout:     dailTimeout,
		clientAndServer: make(map[string]*rpc.Client),
		metadata:        make(map[string]string),
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (this *EtcdV3ClientSelector) start() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:              this.EtcdServers,
		DialTimeout: 		this.DialTimeout,
		Username: 		this.Username,
		Password:  		this.Password,
	})

	if err != nil {
		log.Fatal("etcd new client failed. " + err.Error())
		return
	}
	this.KeysAPI = cli
	this.pullServers()

	// s.ticker = time.NewTicker(s.sessionTimeout)
	// go func() {
	// 	for range s.ticker.C {
	// 		s.pullServers()
	// 	}
	// }()

	go this.watch()
}


func (this *EtcdV3ClientSelector) watch() {
	watcher := this.KeysAPI.Watch(context.Background(),this.BasePath, clientv3.WithPrefix())
	for wresp := range watcher {
		for _,ev := range wresp.Events {
			fmt.Printf("%s %q:%q\n",ev.Type,ev.Kv.Key,ev.Kv.Value)
			if ev.Type == "PUT" || ev.Type == "DELETE" {
				this.pullServers()
				if ev.Kv.Value != "dir" {
					removedServer := strings.TrimPrefix(ev.Kv.Key, this.BasePath+"/")
					this.clientRWMutex.Lock()
					delete(this.clientAndServer,removedServer)
					this.clientRWMutex.Unlock()

				}
			}
		}
	}
}

func (this *EtcdV3ClientSelector) pullServers() {
	resp, err := this.KeysAPI.Get(context.TODO(), this.BasePath, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))

	if err == nil && resp.Kvs != nil {
		if len(resp.Kvs) > 0 {
			var servers []string
			for _, n := range resp.Kvs {
				servers = append(servers, strings.TrimPrefix(n.Key, this.BasePath+"/"))

			}
			this.Servers = servers

			this.createWeighted(resp.Kvs)

			//set weight based on ICMP result
			if this.SelectMode == rpcx.WeightedICMP {
				for _, w := range this.WeightedServers {
					server := w.Server.(string)
					ss := strings.Split(server, "@")
					host, _, _ := net.SplitHostPort(ss[1])
					rtt, _ := Ping(host)
					rtt = CalculateWeight(rtt)
					w.Weight = rtt
					w.EffectiveWeight = rtt
				}
			}

			this.len = len(this.Servers)

			if this.len > 0 {
				this.currentServer = this.currentServer % this.len
			}
		} else {
			// when the last instance is down, it should be deleted
			this.clientAndServer = map[string]*rpc.Client{}
		}

	}
}


func (this *EtcdV3ClientSelector) createWeighted(nodes []*mvccpb.KeyValue) {
	this.WeightedServers = make([]*Weighted, len(this.Servers))

	var inactiveServers []int

	for i, n := range nodes {
		key := strings.TrimPrefix(n.Key, this.BasePath+"/")
		this.WeightedServers[i] = &Weighted{Server: key, Weight: 1, EffectiveWeight: 1}
		this.metadata[key] = n.Value
		if v, err := url.ParseQuery(n.Value); err == nil {
			w := v.Get("weight")
			state := v.Get("state")
			group := v.Get("group")
			if (state != "" && state != "active") || (this.Group != group) {
				inactiveServers = append(inactiveServers, i)
			}

			if w != "" {
				if weight, err := strconv.Atoi(w); err == nil {
					this.WeightedServers[i].Weight = weight
					this.WeightedServers[i].EffectiveWeight = weight
				}
			}
		}
	}

	this.removeInactiveServers(inactiveServers)
}


func (this *EtcdV3ClientSelector) removeInactiveServers(inactiveServers []int) {
	i := len(inactiveServers) - 1
	for ; i >= 0; i-- {
		k := inactiveServers[i]
		removedServer := this.Servers[k]
		this.Servers = append(this.Servers[0:k], this.Servers[k+1:]...)
		this.WeightedServers = append(this.WeightedServers[0:k], this.WeightedServers[k+1:]...)
		this.clientRWMutex.RLock()
		c := this.clientAndServer[removedServer]
		this.clientRWMutex.RUnlock()
		if c != nil {
			this.clientRWMutex.Lock()
			delete(this.clientAndServer, removedServer)
			this.clientRWMutex.Unlock()
			c.Close() //close connection to inactive server
		}
	}
}


func (this *EtcdV3ClientSelector) getCachedClient(server string, clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
	this.clientRWMutex.RLock()
	c := this.clientAndServer[server]
	this.clientRWMutex.RUnlock()
	if c != nil {
		return c, nil
	}
	ss := strings.Split(server, "@") //
	c, err := rpcx.NewDirectRPCClient(this.Client, clientCodecFunc, ss[0], ss[1], this.dailTimeout)
	this.clientRWMutex.Lock()
	this.clientAndServer[server] = c
	this.clientRWMutex.Unlock()
	return c, err
}


func (this *EtcdV3ClientSelector) HandleFailedClient(client *rpc.Client) {
	for k, v := range this.clientAndServer {
		if v == client {
			this.clientRWMutex.Lock()
			delete(this.clientAndServer, k)
			this.clientRWMutex.Unlock()
		}
		client.Close()
		break
	}
}

// Select returns a rpc client
func (this *EtcdV3ClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if this.len == 0 {
		return nil, errors.New("No available service")
	}

	switch this.SelectMode {
	case rpcx.RandomSelect:
		this.currentServer = this.rnd.Intn(this.len)
		server := this.Servers[this.currentServer]
		return this.getCachedClient(server, clientCodecFunc)

	case rpcx.RoundRobin:
		this.currentServer = (this.currentServer + 1) % this.len //not use lock for performance so it is not precise even
		server := this.Servers[this.currentServer]
		return this.getCachedClient(server, clientCodecFunc)

	case rpcx.ConsistentHash:
		if this.HashServiceAndArgs == nil {
			this.HashServiceAndArgs = JumpConsistentHash
		}
		this.currentServer = this.HashServiceAndArgs(this.len, options)
		server := this.Servers[this.currentServer]
		return this.getCachedClient(server, clientCodecFunc)

	case rpcx.WeightedRoundRobin, rpcx.WeightedICMP:
		server := nextWeighted(this.WeightedServers).Server.(string)
		return this.getCachedClient(server, clientCodecFunc)

	case rpcx.Closest:
		closestServers := getClosestServer(this.Latitude, this.Longitude, this.metadata)
		selected := this.rnd.Intn(len(closestServers))
		return this.getCachedClient(closestServers[selected], clientCodecFunc)

	default:
		return nil, errors.New("not supported SelectMode: " + this.SelectMode.String())
	}
}
