// +build quic

package server

import (
	"errors"
	"net"

	"github.com/smallnest/quick"
)

func init() {
	makeListeners["quic"] = quicMakeListener
}

func quicMakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.tlsConfig == nil {
		return nil, errors.New("TLSConfig must be configured in server.Options")
	}

	if len(s.tlsConfig.NextProtos) == 0 {
		s.tlsConfig.NextProtos = []string{"rpcx"}
	}

	return quick.Listen("udp", address, s.tlsConfig, nil)
}
