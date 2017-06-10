// +build linux darwin dragonfly freebsd netbsd openbsd rumprun

package rpcx

import (
	"net"

	reuseport "github.com/kavu/go_reuseport"
	kcp "github.com/xtaci/kcp-go"
)

// block can be nil if the caller wishes to skip encryption.
func makeListener(network, address string, block kcp.BlockCrypt) (ln net.Listener, err error) {
	switch network {
	case "kcp":
		ln, err = kcp.ListenWithOptions(address, block, 10, 3)
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
