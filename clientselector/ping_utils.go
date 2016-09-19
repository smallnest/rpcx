package clientselector

import (
	"net"
	"time"

	fastping "github.com/tatsushid/go-fastping"
)

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

	return
}

func CalculateWeight(rtt int) int {
	switch {
	case rtt > 0 && rtt <= 10:
		return 191
	case rtt > 10 && rtt <= 200:
		return 201 - rtt
	case rtt > 100 && rtt < 1000:
		return 1
	default:
		return 0
	}
}
