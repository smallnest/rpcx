package clientselector

import (
	"errors"
	"math/rand"
	"time"

	"github.com/smallnest/rpcx"
)

// ServerPair is
type ServerPair struct {
	Network, Address string
}

// MultiClientSelector is used to select a direct rpc server from a list.
type MultiClientSelector struct {
	Servers            []ServerPair
	SelectMode         rpcx.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []ServerPair, sm rpcx.SelectMode, dailTimeout time.Duration) *MultiClientSelector {
	s := &MultiClientSelector{
		Servers:     servers,
		SelectMode:  sm,
		dailTimeout: dailTimeout,
		rnd:         rand.New(rand.NewSource(time.Now().UnixNano())),
		len:         len(servers)}

	s.currentServer = s.rnd.Intn(s.len)
	return s
}

func (s *MultiClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

func (s *MultiClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

func (s *MultiClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpcx.ClientConn {
	var clients []*rpcx.ClientConn

	for _, sv := range s.Servers {
		c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, sv.Network, sv.Address, s.dailTimeout)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

//Select returns a rpc client
func (s *MultiClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpcx.ClientConn, error) {
	if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.dailTimeout)

	} else if s.SelectMode == rpcx.RoundRobin {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.dailTimeout)
	} else if s.SelectMode == rpcx.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options...)
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.dailTimeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
}
