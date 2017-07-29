// +build linux darwin dragonfly freebsd netbsd openbsd rumprun

package rpcx

import (
	"crypto/tls"
	"net"

	reuseport "github.com/kavu/go_reuseport"
	quicconn "github.com/marten-seemann/quic-conn"
	kcp "github.com/xtaci/kcp-go"
)

// block can be nil if the caller wishes to skip encryption in kcp.
// tlsConfig can be nil iff we are not using network "quic".
func makeListener(network, address string, block kcp.BlockCrypt, tlsConfig *tls.Config) (ln net.Listener, err error) {
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
	case "quic":
		ln, err = quicconn.Listen("udp", address, tlsConfig)
	default: //tcp
		ln, err = net.Listen(network, address)
	}

	return ln, err
}
