// +build linux darwin dragonfly freebsd netbsd openbsd rumprun

package rpcx

import (
	"net"

	reuseport "github.com/kavu/go_reuseport"
	kcp "github.com/xtaci/kcp-go"
)

func makeListener(network, address string) (ln net.Listener, err error) {
	switch network {
	case "kcp":
		ln, err = kcp.ListenWithOptions(address, nil, 10, 3)
	case "reuseport":
		if validIP4(address) {
			network = "tcp4"
		} else {
			network = "tcp6"
		}

		ln, err = reuseport.NewReusablePortListener(network, address)
	default: //tcp
		ln, err = net.Listen(network, address)
	}

	return ln, err
}
