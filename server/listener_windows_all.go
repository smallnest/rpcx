// +build windows
// +build udp

package server

import (
	"crypto/tls"
	"errors"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
	kcp "github.com/xtaci/kcp-go"
)

// block can be nil if the caller wishes to skip encryption.
// tlsConfig can be nil if we are not using network "quic".
func (s *Server) makeListener(network, address string) (ln net.Listener, err error) {
	switch network {
	case "kcp":
		if s.Options == nil || s.Options["BlockCrypt"] == nil {
			return nil, errors.New("KCP BlockCrypt must be configured in server.Options")
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
		if s.TLSConfig == nil {
			return nil, errors.New("KCP BlockCrypt must be configured in server.Options")
		}
		ln, err = quicconn.Listen("udp", address, s.TLSConfig)
	default: //tcp
		if s.TLSConfig == nil {
			ln, err = net.Listen(network, address)
		} else {
			ln, err = tls.Listen(network, address, s.TLSConfig)
		}
	}

	return ln, err
}
