package clientselector

import (
	"errors"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/core"
)

// ServerPeer is
type ServerPeer struct {
	Network, Address string
	Weight           int
}

// MultiClientSelector is used to select a direct rpc server from a list.
type MultiClientSelector struct {
	Servers            []*ServerPeer
	clientAndServer    map[string]*core.Client
	clientRWMutex      sync.RWMutex
	WeightedServers    []*Weighted
	SelectMode         rpcx.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []*ServerPeer, sm rpcx.SelectMode, dailTimeout time.Duration) *MultiClientSelector {
	s := &MultiClientSelector{
		Servers:         servers,
		clientAndServer: make(map[string]*core.Client),
		SelectMode:      sm,
		dailTimeout:     dailTimeout,
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano())),
		len:             len(servers)}

	if sm == rpcx.WeightedRoundRobin || sm == rpcx.WeightedICMP {
		s.WeightedServers = make([]*Weighted, len(s.Servers))
		for i, ss := range s.Servers {
			s.WeightedServers[i] = &Weighted{Server: ss, Weight: ss.Weight, EffectiveWeight: ss.Weight}
		}
	}

	//set weight based on ICMP result
	if sm == rpcx.WeightedICMP {
		for _, w := range s.WeightedServers {
			server := w.Server.(*ServerPeer)
			ss := strings.Split(server.Address, "@")
			host, _, _ := net.SplitHostPort(ss[1])
			rtt, _ := Ping(host)
			rtt = CalculateWeight(rtt)
			w.Weight = rtt
			w.EffectiveWeight = rtt
		}
	}

	s.currentServer = s.rnd.Intn(s.len)
	return s
}

//SetClient set a Client in order that clientSelector can uses it
func (s *MultiClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//SetSelectMode sets SelectMode
func (s *MultiClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

//AllClients returns core.Clients to all servers
func (s *MultiClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*core.Client {
	var clients []*core.Client

	for _, sv := range s.Servers {
		c, err := s.getCachedClient(sv.Network, sv.Address, clientCodecFunc)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

func (s *MultiClientSelector) getCachedClient(network string, address string, clientCodecFunc rpcx.ClientCodecFunc) (*core.Client, error) {
	key := network + "@" + address
	s.clientRWMutex.RLock()
	c := s.clientAndServer[key]
	s.clientRWMutex.RUnlock()
	if c != nil {
		return c, nil

	}
	c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, network, address, s.dailTimeout)

	s.clientRWMutex.Lock()
	s.clientAndServer[key] = c
	s.clientRWMutex.Unlock()
	return c, err
}

func (s *MultiClientSelector) HandleFailedClient(client *core.Client) {
	if rpcx.Reconnect != nil && rpcx.Reconnect(client, s.clientAndServer, s.Client, s.dailTimeout) {
		return
	}

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
func (s *MultiClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*core.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	switch s.SelectMode {
	case rpcx.RandomSelect:
		s.currentServer = s.rnd.Intn(s.len)
		peer := s.Servers[s.currentServer]
		return s.getCachedClient(peer.Network, peer.Address, clientCodecFunc)

	case rpcx.RoundRobin:
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		peer := s.Servers[s.currentServer]
		return s.getCachedClient(peer.Network, peer.Address, clientCodecFunc)

	case rpcx.ConsistentHash:
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options...)
		peer := s.Servers[s.currentServer]
		return s.getCachedClient(peer.Network, peer.Address, clientCodecFunc)

	case rpcx.WeightedRoundRobin, rpcx.WeightedICMP:
		best := nextWeighted(s.WeightedServers)
		peer := best.Server.(*ServerPeer)
		return s.getCachedClient(peer.Network, peer.Address, clientCodecFunc)

	default:
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
