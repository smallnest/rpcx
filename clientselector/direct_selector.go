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
	Servers       []ServerPair
	SelectMode    rpcx.SelectMode
	timeout       time.Duration
	rnd           *rand.Rand
	currentServer int
	len           int
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []ServerPair, sm rpcx.SelectMode, timeout time.Duration) *MultiClientSelector {
	return &MultiClientSelector{
		Servers:    servers,
		SelectMode: sm,
		timeout:    timeout,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
		len:        len(servers)}
}

//Select returns a rpc client
func (s *MultiClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
	if s.SelectMode == rpcx.RandomSelect {
		pair := s.Servers[s.rnd.Intn(s.len)]
		return rpcx.NewDirectRPCClient(clientCodecFunc, pair.Network, pair.Address, s.timeout)

	} else if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		pair := s.Servers[s.currentServer]
		return rpcx.NewDirectRPCClient(clientCodecFunc, pair.Network, pair.Address, s.timeout)

	} else {
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
