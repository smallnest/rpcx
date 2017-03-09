package clientselector

import (
	"errors"
	"math/rand"
	"net"
	"net/rpc"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/saiser/rpcx"
	"github.com/saiser/rpcx/log"
	"golang.org/x/net/context"
)

// EtcdClientSelector is used to select a rpc server from etcd.
type EtcdClientSelector struct {
	EtcdServers        []string
	KeysAPI            client.KeysAPI
	sessionTimeout     time.Duration
	BasePath           string //should endwith serviceName
	Servers            []string
	Group              string
	clientAndServer    map[string]*rpc.Client
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
	Username           string
	Password           string
}

// NewEtcdClientSelector creates a EtcdClientSelector
func NewEtcdClientSelector(etcdServers []string, basePath string, sessionTimeout time.Duration, sm rpcx.SelectMode, dailTimeout time.Duration, username string, password string) *EtcdClientSelector {
	selector := &EtcdClientSelector{
		EtcdServers:     etcdServers,
		BasePath:        basePath,
		sessionTimeout:  sessionTimeout,
		SelectMode:      sm,
		dailTimeout:     dailTimeout,
		clientAndServer: make(map[string]*rpc.Client),
		metadata:        make(map[string]string),
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano())),
		Username:        username,
		Password:        password,
	}

	selector.start()
	return selector
}

//SetClient set a Client in order that clientSelector can uses it
func (s *EtcdClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//SetSelectMode sets SelectMode
func (s *EtcdClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

//AllClients returns rpc.Clients to all servers
func (s *EtcdClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv, "@")
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
		if err != nil {
			log.Fatalf("rpc client connect server failed: %v", err.Error())
			continue
		} else {
			clients = append(clients, c)
		}
	}

	return clients
}

func (s *EtcdClientSelector) start() {
	cli, err := client.New(client.Config{
		Endpoints:               s.EtcdServers,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: s.sessionTimeout,
	})

	if err != nil {
		log.Fatal("etcd new client failed: %v", err.Error())
		return
	}
	s.KeysAPI = client.NewKeysAPI(cli)
	s.pullServers()

	go s.watch()
}

func (s *EtcdClientSelector) watch() {
	watcher := s.KeysAPI.Watcher(s.BasePath, &client.WatcherOptions{
		Recursive: true,
	})

	for {
		res, err := watcher.Next(context.Background())
		if err != nil {
			break
		}

		//services are changed, we pull service again instead of processing single node
		if res.Action == "expire" {
			s.pullServers()
			if !res.Node.Dir {
				// clientAndServer delete the invalid client connection
				removedServer := strings.TrimPrefix(res.Node.Key, s.BasePath+"/")
				s.clientRWMutex.Lock()
				delete(s.clientAndServer, removedServer)
				s.clientRWMutex.Unlock()
			}
		} else if res.Action == "set" || res.Action == "update" {
			s.pullServers()
		} else if res.Action == "delete" {
			s.pullServers()
		}
	}
}

func (s *EtcdClientSelector) pullServers() {
	resp, err := s.KeysAPI.Get(context.TODO(), s.BasePath, &client.GetOptions{
		Recursive: true,
		Sort:      true,
	})

	if err == nil && resp.Node != nil {
		if len(resp.Node.Nodes) > 0 {
			var servers []string
			for _, n := range resp.Node.Nodes {
				servers = append(servers, strings.TrimPrefix(n.Key, s.BasePath+"/"))

			}
			s.Servers = servers

			s.createWeighted(resp.Node.Nodes)

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
			s.clientAndServer = map[string]*rpc.Client{}
		}

	}
}

func (s *EtcdClientSelector) createWeighted(nodes client.Nodes) {
	s.WeightedServers = make([]*Weighted, len(s.Servers))

	var inactiveServers []int

	for i, n := range nodes {
		key := strings.TrimPrefix(n.Key, s.BasePath+"/")
		s.WeightedServers[i] = &Weighted{Server: key, Weight: 1, EffectiveWeight: 1}
		s.metadata[key] = n.Value
		if v, err := url.ParseQuery(n.Value); err == nil {
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

func (s *EtcdClientSelector) removeInactiveServers(inactiveServers []int) {
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

func (s *EtcdClientSelector) getCachedClient(server string, clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
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

func (s *EtcdClientSelector) HandleFailedClient(client *rpc.Client) {
	for k, v := range s.clientAndServer {
		if v == client {
			s.clientRWMutex.Lock()
			delete(s.clientAndServer, k)
			s.clientRWMutex.Unlock()
		}
		client.Close()
		break
	}
}

// Select returns a rpc client
func (s *EtcdClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
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
