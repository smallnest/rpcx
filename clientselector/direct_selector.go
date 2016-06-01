package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
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
	timeout            time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []ServerPair, sm rpcx.SelectMode, timeout time.Duration) *MultiClientSelector {
	s := &MultiClientSelector{
		Servers:    servers,
		SelectMode: sm,
		timeout:    timeout,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
		len:        len(servers)}

	s.currentServer = s.rnd.Intn(s.len)
	return s
}

func (s *MultiClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//Select returns a rpc client
func (s *MultiClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.timeout)

	} else if s.SelectMode == rpcx.RoundRobin {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.timeout)
	} else if s.SelectMode == rpcx.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options...)
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, pair.Network, pair.Address, s.timeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
}
