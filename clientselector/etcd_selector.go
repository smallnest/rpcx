package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/smallnest/rpcx"
)

// EtcdClientSelector is used to select a rpc server from etcd.
type EtcdClientSelector struct {
	EtcdServers        []string
	KeysAPI            client.KeysAPI
	ticker             *time.Ticker
	sessionTimeout     time.Duration
	BasePath           string //should endwith serviceName
	Servers            []string
	SelectMode         rpcx.SelectMode
	timeout            time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewEtcdClientSelector creates a EtcdClientSelector
func NewEtcdClientSelector(etcdServers []string, basePath string, sessionTimeout time.Duration, sm rpcx.SelectMode, timeout time.Duration) *EtcdClientSelector {
	selector := &EtcdClientSelector{
		EtcdServers:    etcdServers,
		BasePath:       basePath,
		sessionTimeout: sessionTimeout,
		SelectMode:     sm,
		timeout:        timeout,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *EtcdClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

func (s *EtcdClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

func (s *EtcdClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv, "@")
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)
		if err == nil {
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
		return
	}
	s.KeysAPI = client.NewKeysAPI(cli)
	s.pullServers()

	s.ticker = time.NewTicker(s.sessionTimeout)
	go func() {
		for range s.ticker.C {
			s.pullServers()
		}
	}()
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
			s.len = len(servers)
			s.currentServer = s.currentServer % s.len
		}
	}
}

//Select returns a rpc client
func (s *EtcdClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //tcp@ip , tcp4@ip or tcp6@ip
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == rpcx.RoundRobin {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == rpcx.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())

}
