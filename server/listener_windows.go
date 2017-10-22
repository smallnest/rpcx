// +build windows
// +build !udp

package server

import (
	"errors"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
)

// block can be nil if the caller wishes to skip encryption.
// tlsConfig can be nil iff we are not using network "quic".
func (s *Server) makeListener(network, address string) (ln net.Listener, err error) {
	switch network {
	case "reuseport":
		if validIP4(address) {
			network = "tcp4"
		} else {
			network = "tcp6"
		}

		ln, err = net.Listen(network, address)
	default: //tcp
		if s.TLSConfig == nil {
			return nil, errors.New("KCP BlockCrypt must be configured in server.Options")
		}
		ln, err = quicconn.Listen("udp", address, s.TLSConfig)
	}

	return ln, err
}
