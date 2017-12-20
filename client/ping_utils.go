// +build ping

package client

import (
	"context"
	"net"
	"strings"
	"time"

	fastping "github.com/tatsushid/go-fastping"
)

func newWeightedICMPSelector(servers map[string]string) Selector {
	ss := createICMPWeighted(servers)
	return &weightedICMPSelector{servers: ss}
}

func (s weightedICMPSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
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

func (s *weightedICMPSelector) UpdateServer(servers map[string]string) {
	ss := createICMPWeighted(servers)
	s.servers = ss
}

func createICMPWeighted(servers map[string]string) []*Weighted {
	var ss = make([]*Weighted, 0, len(servers))
	for k := range servers {
		w := &Weighted{Server: k, Weight: 1, EffectiveWeight: 1}
		server := strings.Split(k, "@")
		host, _, _ := net.SplitHostPort(server[1])
		rtt, _ := Ping(host)
		rtt = CalculateWeight(rtt)
		w.Weight = rtt
		w.EffectiveWeight = rtt
		ss = append(ss, w)
	}
	return ss
}

// Ping gets network traffic by ICMP
func Ping(host string) (rtt int, err error) {
	rtt = 1000 //default and timeout is 1000 ms

	p := fastping.NewPinger()
	p.Network("udp")
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		return 0, err
	}
	p.AddIPAddr(ra)

	p.OnRecv = func(addr *net.IPAddr, r time.Duration) {
		rtt = int(r.Nanoseconds() / 1000000)
	}
	// p.OnIdle = func() {

	// }
	err = p.Run()

	return rtt, err
}

// CalculateWeight converts the rtt to weighted by:
//  1. weight=191 if t <= 10
//  2. weight=201 -t if 10 < t <=200
//  3. weight=1 if 200 < t < 1000
//  4. weight = 0 if t >= 1000
//
// It means servers that ping time t < 10 will be preferred
// and servers won't be selected if t > 1000.
// It is hard coded based on Ops experience.
func CalculateWeight(rtt int) int {
	switch {
	case rtt >= 0 && rtt <= 10:
		return 191
	case rtt > 10 && rtt <= 200:
		return 201 - rtt
	case rtt > 100 && rtt < 1000:
		return 1
	default:
		return 0
	}
}
