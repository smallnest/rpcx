// +build !windows
// +build udp

package server

import (
	"net"

	reuseport "github.com/kavu/go_reuseport"
	quicconn "github.com/marten-seemann/quic-conn"
	kcp "github.com/xtaci/kcp-go"
)

// block can be nil if the caller wishes to skip encryption in kcp.
// tlsConfig can be nil iff we are not using network "quic".
func (s *Server) makeListener(network, address string) (ln net.Listener, err error) {
	switch network {
	case "kcp":
		ln, err = kcp.ListenWithOptions(address, s.KCPConfig.BlockCrypt, 10, 3)
	case "reuseport":
		if validIP4(address) {
			network = "tcp4"
		} else {
			network = "tcp6"
		}

		ln, err = reuseport.NewReusablePortListener(network, address)
	case "quic":
		ln, err = quicconn.Listen("udp", address, s.QUICConfig.TlsConfig)
	default: //tcp, http
		ln, err = net.Listen(network, address)
	}

	return ln, err
}
