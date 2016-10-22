package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"net/url"
	"strconv"
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
	clientAndServer    map[string]*rpc.Client
	WeightedServers    []*Weighted
	ServiceName        string
	SelectMode         rpcx.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewConsulClientSelector creates a ConsulClientSelector
func NewConsulClientSelector(consulAddress string, serviceName string, sessionTimeout time.Duration, sm rpcx.SelectMode, dailTimeout time.Duration) *ConsulClientSelector {
	selector := &ConsulClientSelector{
		ConsulAddress:   consulAddress,
		ServiceName:     serviceName,
		Servers:         make([]*api.AgentService, 1),
		clientAndServer: make(map[string]*rpc.Client),
		sessionTimeout:  sessionTimeout,
		SelectMode:      sm,
		dailTimeout:     dailTimeout,
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

//SetClient set a Client in order that clientSelector can uses it
func (s *ConsulClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//SetSelectMode sets SelectMode
func (s *ConsulClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

//AllClients returns rpc.Clients to all servers
func (s *ConsulClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv.Address, "@")
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
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

func (s *ConsulClientSelector) createWeighted(ass map[string]*api.AgentService) {
	s.WeightedServers = make([]*Weighted, len(s.Servers))

	i := 0
	for k, v := range ass {
		if strings.HasPrefix(k, s.ServiceName) {
			s.WeightedServers[i] = &Weighted{Server: v, Weight: 1, EffectiveWeight: 1}
			i++
			if len(v.Tags) > 0 {
				if values, err := url.ParseQuery(v.Tags[0]); err == nil {
					w := values.Get("weight")
					if w != "" {
						weight, err := strconv.Atoi(w)
						if err != nil {
							s.WeightedServers[i].Weight = weight
							s.WeightedServers[i].EffectiveWeight = weight
						}
					}
				}
			}

		}
	}

}

func (s *ConsulClientSelector) getCachedClient(server string, clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
	c := s.clientAndServer[server]
	if c != nil {
		return c, nil
	}
	ss := strings.Split(server, "@") //
	c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
	s.clientAndServer[server] = c
	return c, err
}

// Select returns a rpc client
func (s *ConsulClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	switch s.SelectMode {
	case rpcx.RandomSelect:
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server.Address, clientCodecFunc)

	case rpcx.RoundRobin:
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server.Address, clientCodecFunc)

	case rpcx.ConsistentHash:
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server.Address, clientCodecFunc)

	case rpcx.WeightedRoundRobin:
		server := nextWeighted(s.WeightedServers).Server.(*api.AgentService)
		return s.getCachedClient(server.Address, clientCodecFunc)

	default:
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
