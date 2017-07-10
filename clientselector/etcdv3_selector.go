package clientselector

import (
	"errors"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	mvccpb "github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/core"
	"github.com/smallnest/rpcx/log"
	"golang.org/x/net/context"
)

// EtcdV3ClientSelector is used to select a rpc server from etcd via V3 API.
type EtcdV3ClientSelector struct {
	EtcdServers        []string
	KeysAPI            *clientv3.Client
	sessionTimeout     time.Duration
	BasePath           string //should endwith serviceName
	Servers            []string
	Group              string
	clientAndServer    map[string]*core.Client
	clientRWMutex      sync.RWMutex
	metadata           map[string]string
	Latitude           float64
	Longitude          float64
	WeightedServers    []*Weighted
	SelectMode         rpcx.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
	DialTimeout        time.Duration
	Username           string
	Password           string
	UpdateIntervalNum  int64
}

//SetClient set a Client in order that clientSelector can uses it
func (s *EtcdV3ClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//SetSelectMode sets SelectMode
func (s *EtcdV3ClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

//AllClients returns core.Clients to all servers
func (s *EtcdV3ClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*core.Client {
	var clients []*core.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv, "@")
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
		if err != nil {
			log.Errorf("rpc client connect server failed: %v", err.Error())
			continue
		} else {
			clients = append(clients, c)
		}
	}

	return clients
}

// NewEtcdClientSelector creates a EtcdClientSelector
func NewEtcdV3ClientSelector(etcdServers []string, servicePath string, sessionTimeout time.Duration, sm rpcx.SelectMode, dailTimeout time.Duration) *EtcdV3ClientSelector {
	selector := &EtcdV3ClientSelector{
		EtcdServers:     etcdServers,
		BasePath:        servicePath,
		sessionTimeout:  sessionTimeout,
		SelectMode:      sm,
		dailTimeout:     dailTimeout,
		clientAndServer: make(map[string]*core.Client),
		metadata:        make(map[string]string),
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *EtcdV3ClientSelector) start() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   s.EtcdServers,
		DialTimeout: s.DialTimeout,
		Username:    s.Username,
		Password:    s.Password,
	})

	if err != nil {
		log.Errorf("etcd new client failed: %v", err.Error())
		return
	}
	s.KeysAPI = cli
	s.pullServers()

	go s.watch()
}

func (s *EtcdV3ClientSelector) watch() {
	watcher := s.KeysAPI.Watch(context.Background(), s.BasePath, clientv3.WithPrefix())
	for wresp := range watcher {
		for _, ev := range wresp.Events {
			//log.Infof("%s %q:%q\n",ev.Type,ev.Kv.Key,ev.Kv.Value)
			s.pullServers()
			removedServer := strings.TrimPrefix(string(ev.Kv.Key), s.BasePath+"/")
			s.clientRWMutex.Lock()
			delete(s.clientAndServer, removedServer)
			s.clientRWMutex.Unlock()
		}
	}
}

func (s *EtcdV3ClientSelector) pullServers() {
	resp, err := s.KeysAPI.Get(context.TODO(), s.BasePath, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))

	if err == nil && resp.Kvs != nil {
		if len(resp.Kvs) > 0 {
			var servers []string
			for _, n := range resp.Kvs {
				servers = append(servers, strings.TrimPrefix(string(n.Key), s.BasePath+"/"))

			}
			s.Servers = servers

			s.createWeighted(resp.Kvs)

			//set weight based on ICMP result
			if s.SelectMode == rpcx.WeightedICMP {
				for _, w := range s.WeightedServers {
					server := w.Server.(string)
					ss := strings.Split(server, "@")
					host, _, _ := net.SplitHostPort(ss[1])
					rtt, _ := Ping(host)
					rtt = CalculateWeight(rtt)
					w.Weight = rtt
					w.EffectiveWeight = rtt
				}
			}

			s.len = len(s.Servers)

			if s.len > 0 {
				s.currentServer = s.currentServer % s.len
			}
		} else {
			// when the last instance is down, it should be deleted
			s.clientAndServer = map[string]*core.Client{}
		}

	}
}

func (s *EtcdV3ClientSelector) createWeighted(nodes []*mvccpb.KeyValue) {
	s.WeightedServers = make([]*Weighted, len(s.Servers))

	var inactiveServers []int

	for i, n := range nodes {
		key := strings.TrimPrefix(string(n.Key), s.BasePath+"/")
		s.WeightedServers[i] = &Weighted{Server: key, Weight: 1, EffectiveWeight: 1}
		s.metadata[key] = string(n.Value)
		if v, err := url.ParseQuery(string(n.Value)); err == nil {
			w := v.Get("weight")
			state := v.Get("state")
			group := v.Get("group")
			if (state != "" && state != "active") || (s.Group != group) {
				inactiveServers = append(inactiveServers, i)
			}

			if w != "" {
				if weight, err := strconv.Atoi(w); err == nil {
					s.WeightedServers[i].Weight = weight
					s.WeightedServers[i].EffectiveWeight = weight
				}
			}
		}
	}

	s.removeInactiveServers(inactiveServers)
}

func (s *EtcdV3ClientSelector) removeInactiveServers(inactiveServers []int) {
	i := len(inactiveServers) - 1
	for ; i >= 0; i-- {
		k := inactiveServers[i]
		removedServer := s.Servers[k]
		s.Servers = append(s.Servers[0:k], s.Servers[k+1:]...)
		s.WeightedServers = append(s.WeightedServers[0:k], s.WeightedServers[k+1:]...)
		s.clientRWMutex.RLock()
		c := s.clientAndServer[removedServer]
		s.clientRWMutex.RUnlock()
		if c != nil {
			s.clientRWMutex.Lock()
			delete(s.clientAndServer, removedServer)
			s.clientRWMutex.Unlock()
			c.Close() //close connection to inactive server
		}
	}
}

func (s *EtcdV3ClientSelector) getCachedClient(server string, clientCodecFunc rpcx.ClientCodecFunc) (*core.Client, error) {
	s.clientRWMutex.RLock()
	c := s.clientAndServer[server]
	s.clientRWMutex.RUnlock()
	if c != nil {
		return c, nil
	}
	ss := strings.Split(server, "@") //
	c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
	s.clientRWMutex.Lock()
	s.clientAndServer[server] = c
	s.clientRWMutex.Unlock()
	return c, err
}

func (s *EtcdV3ClientSelector) HandleFailedClient(client *core.Client) {
	if rpcx.Reconnect != nil && rpcx.Reconnect(client, s.clientAndServer, s.Client, s.dailTimeout) {
		return
	}

	var foundC string
	s.clientRWMutex.RLock()
	for k, v := range s.clientAndServer {
		if v == client {
			foundC = k
		}
		break
	}
	s.clientRWMutex.RUnlock()

	if foundC != "" {
		s.clientRWMutex.Lock()
		delete(s.clientAndServer, foundC)
		client.Close()
		s.clientRWMutex.Unlock()
	}
}

// Select returns a rpc client
func (s *EtcdV3ClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*core.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	switch s.SelectMode {
	case rpcx.RandomSelect:
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.RoundRobin:
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.ConsistentHash:
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.WeightedRoundRobin, rpcx.WeightedICMP:
		server := nextWeighted(s.WeightedServers).Server.(string)
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.Closest:
		closestServers := getClosestServer(s.Latitude, s.Longitude, s.metadata)
		selected := s.rnd.Intn(len(closestServers))
		return s.getCachedClient(closestServers[selected], clientCodecFunc)

	default:
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
