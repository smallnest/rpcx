package client

import (
	"context"
	"math/rand"
	"time"
)

// Selector defines selector that selects one service from candidates.
type Selector interface {
	Select(ctx context.Context, servicePath, serviceMethod string) string
	UpdateServer(servers map[string]string)
}

func newSelector(selectMode SelectMode, servers map[string]string) Selector {
	switch selectMode {
	case RandomSelect:
		return newRandomSelector(servers)
	default:
		return newRandomSelector(servers)
	}
}

// randomSelector selects randomly.
type randomSelector struct {
	servers []string
	r       *rand.Rand
}

func newRandomSelector(servers map[string]string) Selector {
	var ss []string
	for k := range servers {
		ss = append(ss, k)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	return &randomSelector{servers: ss, r: r}
}

func (s randomSelector) Select(ctx context.Context, servicePath, serviceMethod string) string {
	ss := s.servers
	i := s.r.Intn(len(ss))
	return ss[i]
}

func (s *randomSelector) UpdateServer(servers map[string]string) {
	var ss []string
	for k := range servers {
		ss = append(ss, k)
	}

	s.servers = ss
}
