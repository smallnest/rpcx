package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"time"

	"github.com/smallnest/betterrpc"
)

// ServerPair is
type ServerPair struct {
	Network, Address string
}

// MultiClientSelector is used to select a direct rpc server from a list.
type MultiClientSelector struct {
	Servers       []ServerPair
	SelectMode    betterrpc.SelectMode
	rnd           *rand.Rand
	currentServer int
	len           int
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []ServerPair, sm betterrpc.SelectMode) *MultiClientSelector {
	return &MultiClientSelector{
		Servers:    servers,
		SelectMode: sm,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
		len:        len(servers)}
}

//Select returns a rpc client
func (s *MultiClientSelector) Select(clientCodecFunc betterrpc.ClientCodecFunc) (*rpc.Client, error) {
	if s.SelectMode == betterrpc.RandomSelect {
		pair := s.Servers[s.rnd.Intn(s.len)]
		return betterrpc.NewDirectRPCClient(clientCodecFunc, pair.Network, pair.Address)

	} else if s.SelectMode == betterrpc.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		pair := s.Servers[s.currentServer]
		return betterrpc.NewDirectRPCClient(clientCodecFunc, pair.Network, pair.Address)

	} else {
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
