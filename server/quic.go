// +build quic

package server

import (
	"errors"
	"net"

	quicconn "github.com/marten-seemann/quic-conn"
)

func init() {
	makeListeners["quic"] = quicMakeListener
}

func quicMakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.tlsConfig == nil {
		return nil, errors.New("TLSConfig must be configured in server.Options")
	}
	return quicconn.Listen("udp", address, s.tlsConfig)
}
