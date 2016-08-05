package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/smallnest/rpcx"
)

// ConsulClientSelector is used to select a rpc server from consul.
//This registry is experimental and has not been test.
type ConsulClientSelector struct {
	ConsulAddress      string
	consulConfig       *api.Config
	client             *api.Client
	ticker             *time.Ticker
	sessionTimeout     time.Duration
	Servers            []*api.AgentService
	ServiceName        string
	SelectMode         rpcx.SelectMode
	timeout            time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewConsulClientSelector creates a ConsulClientSelector
func NewConsulClientSelector(consulAddress string, serviceName string, sessionTimeout time.Duration, sm rpcx.SelectMode, timeout time.Duration) *ConsulClientSelector {
	selector := &ConsulClientSelector{
		ConsulAddress:  consulAddress,
		ServiceName:    serviceName,
		Servers:        make([]*api.AgentService, 1),
		sessionTimeout: sessionTimeout,
		SelectMode:     sm,
		timeout:        timeout,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *ConsulClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

func (s *ConsulClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

func (s *ConsulClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv.Address, "@")
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

func (s *ConsulClientSelector) start() {
	if s.consulConfig == nil {
		s.consulConfig = api.DefaultConfig()
		s.consulConfig.Address = s.ConsulAddress
	}
	s.client, _ = api.NewClient(s.consulConfig)

	s.pullServers()

	s.ticker = time.NewTicker(s.sessionTimeout)
	go func() {
		for range s.ticker.C {
			s.pullServers()
		}
	}()
}

func (s *ConsulClientSelector) pullServers() {
	agent := s.client.Agent()
	ass, err := agent.Services()

	if err != nil {
		return
	}

	var services []*api.AgentService
	for k, v := range ass {
		if strings.HasPrefix(k, s.ServiceName) {
			services = append(services, v)
		}
	}
	s.Servers = services
}

//Select returns a rpc client
func (s *ConsulClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@") //tcp@ip , tcp4@ip or tcp6@ip
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@") //
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == rpcx.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@") //
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.timeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())

}
