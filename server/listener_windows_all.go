// +build windows
// +build udp

package rpcx

import (
	"errors"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
	kcp "github.com/xtaci/kcp-go"
)

// block can be nil if the caller wishes to skip encryption.
// tlsConfig can be nil iff we are not using network "quic".
func makeListener(network, address string) (ln net.Listener, err error) {
	switch network {
	case "kcp":
		if s.Options == nil || s.Options["BlockCrypt"] == nil {
			return errors.New("KCP BlockCrypt must be configured in server.Options")
		}

		ln, err = kcp.ListenWithOptions(address, s.Options["BlockCrypt"].(kcp.BlockCrypt), 10, 3)
	case "reuseport":
		if validIP4(address) {
			network = "tcp4"
		} else {
			network = "tcp6"
		}

		ln, err = net.Listen(network, address)
	case "quic":
		ln, err = quicconn.Listen("udp", address, tlsConfig)
	default: //tcp
		ln, err = net.Listen(network, address)
	}

	return ln, err
}
