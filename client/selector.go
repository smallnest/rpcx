package client

import (
	"context"
	"math"
	"math/rand"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/edwingeng/doublejump"

	"github.com/valyala/fastrand"
)

type SelectFunc func(ctx context.Context, servicePath, serviceMethod string, args interface{}) string

// Selector defines selector that selects one service from candidates.
type Selector interface {
	Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string // SelectFunc
	UpdateServer(servers map[string]string)
}

func newSelector(selectMode SelectMode, servers map[string]string) Selector {
	switch selectMode {
	case RandomSelect:
		return newRandomSelector(servers)
	case RoundRobin:
		return newRoundRobinSelector(servers)
	case WeightedRoundRobin:
		return newWeightedRoundRobinSelector(servers)
	case WeightedICMP:
		return newWeightedICMPSelector(servers)
	case ConsistentHash:
		return newConsistentHashSelector(servers)
	case SelectByUser:
		return nil
	default:
		return newRandomSelector(servers)
	}
}

// randomSelector selects randomly.
type randomSelector struct {
	servers []string
}

func newRandomSelector(servers map[string]string) Selector {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		ss = append(ss, k)
	}

	return &randomSelector{servers: ss}
}

func (s randomSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	ss := s.servers
	if len(ss) == 0 {
		return ""
	}
	i := fastrand.Uint32n(uint32(len(ss)))
	return ss[i]
}

func (s *randomSelector) UpdateServer(servers map[string]string) {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		ss = append(ss, k)
	}

	s.servers = ss
}

// roundRobinSelector selects servers with roundrobin.
type roundRobinSelector struct {
	servers []string
	i       int
}

func newRoundRobinSelector(servers map[string]string) Selector {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		ss = append(ss, k)
	}

	return &roundRobinSelector{servers: ss}
}

func (s *roundRobinSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	ss := s.servers
	if len(ss) == 0 {
		return ""
	}
	i := s.i
	i = i % len(ss)
	s.i = i + 1

	return ss[i]
}

func (s *roundRobinSelector) UpdateServer(servers map[string]string) {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		ss = append(ss, k)
	}

	s.servers = ss
}

// weightedRoundRobinSelector selects servers with weighted.
type weightedRoundRobinSelector struct {
	servers []*Weighted
}

func newWeightedRoundRobinSelector(servers map[string]string) Selector {
	ss := createWeighted(servers)
	return &weightedRoundRobinSelector{servers: ss}
}

func (s *weightedRoundRobinSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	ss := s.servers
	if len(ss) == 0 {
		return ""
	}
	w := nextWeighted(ss)
	if w == nil {
		return ""
	}
	return w.Server
}

func (s *weightedRoundRobinSelector) UpdateServer(servers map[string]string) {
	ss := createWeighted(servers)
	s.servers = ss
}

func createWeighted(servers map[string]string) []*Weighted {
	ss := make([]*Weighted, 0, len(servers))
	for k, metadata := range servers {
		w := &Weighted{Server: k, Weight: 1, EffectiveWeight: 1}

		if v, err := url.ParseQuery(metadata); err == nil {
			ww := v.Get("weight")
			if ww != "" {
				if weight, err := strconv.Atoi(ww); err == nil {
					w.Weight = weight
					w.EffectiveWeight = weight
				}
			}
		}

		ss = append(ss, w)
	}

	return ss
}

type geoServer struct {
	Server    string
	Latitude  float64
	Longitude float64
}

// geoSelector selects servers based on location.
type geoSelector struct {
	servers   []*geoServer
	Latitude  float64
	Longitude float64
	r         *rand.Rand
}

func newGeoSelector(servers map[string]string, latitude, longitude float64) Selector {
	ss := createGeoServer(servers)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &geoSelector{servers: ss, Latitude: latitude, Longitude: longitude, r: r}
}

func (s geoSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	if len(s.servers) == 0 {
		return ""
	}

	var server []string
	min := math.MaxFloat64
	for _, gs := range s.servers {
		d := getDistanceFrom(s.Latitude, s.Longitude, gs.Latitude, gs.Longitude)
		if d < min {
			server = []string{gs.Server}
			min = d
		} else if d == min {
			server = append(server, gs.Server)
		}
	}

	if len(server) == 1 {
		return server[0]
	}

	return server[s.r.Intn(len(server))]
}

func (s *geoSelector) UpdateServer(servers map[string]string) {
	ss := createGeoServer(servers)
	s.servers = ss
}

func createGeoServer(servers map[string]string) []*geoServer {
	geoServers := make([]*geoServer, 0, len(servers))

	for s, metadata := range servers {
		if v, err := url.ParseQuery(metadata); err == nil {
			latStr := v.Get("latitude")
			lonStr := v.Get("longitude")

			if latStr == "" || lonStr == "" {
				continue
			}

			lat, err := strconv.ParseFloat(latStr, 64)
			if err != nil {
				continue
			}
			lon, err := strconv.ParseFloat(lonStr, 64)
			if err != nil {
				continue
			}

			geoServers = append(geoServers, &geoServer{Server: s, Latitude: lat, Longitude: lon})

		}
	}

	return geoServers
}

// consistentHashSelector selects based on JumpConsistentHash.
type consistentHashSelector struct {
	h       *doublejump.Hash
	servers []string
}

func newConsistentHashSelector(servers map[string]string) Selector {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		ss = append(ss, k)
	}

	sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })
	return &consistentHashSelector{servers: ss, h: doublejump.NewHash()}
}

func (s consistentHashSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	ss := s.servers
	if len(ss) == 0 {
		return ""
	}

	key := genKey(servicePath, serviceMethod, args)
	selected, _ := s.h.Get(key).(string)
	return selected
}

func (s *consistentHashSelector) UpdateServer(servers map[string]string) {
	ss := make([]string, 0, len(servers))
	for k := range servers {
		s.h.Add(k)
		ss = append(ss, k)
	}

	sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })

	for _, k := range s.servers {
		if servers[k] == "" { // remove
			s.h.Remove(k)
		}
	}
	s.servers = ss
}

// weightedICMPSelector selects servers with ping result.
type weightedICMPSelector struct {
	servers []*Weighted
}
