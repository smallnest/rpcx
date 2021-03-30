package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
)

var makeListeners = make(map[string]MakeListener)

func init() {
	makeListeners["tcp"] = tcpMakeListener("tcp")
	makeListeners["tcp4"] = tcpMakeListener("tcp4")
	makeListeners["tcp6"] = tcpMakeListener("tcp6")
	makeListeners["http"] = tcpMakeListener("tcp")
	makeListeners["ws"] = tcpMakeListener("tcp")
	makeListeners["wss"] = tcpMakeListener("tcp")
}

// RegisterMakeListener registers a MakeListener for network.
func RegisterMakeListener(network string, ml MakeListener) {
	makeListeners[network] = ml
}

// MakeListener defines a listener generator.
type MakeListener func(s *Server, address string) (ln net.Listener, err error)

// block can be nil if the caller wishes to skip encryption in kcp.
// tlsConfig can be nil iff we are not using network "quic".
func (s *Server) makeListener(network, address string) (ln net.Listener, err error) {
	ml := makeListeners[network]
	if ml == nil {
		return nil, fmt.Errorf("can not make listener for %s", network)
	}

	if network == "wss" && s.tlsConfig == nil {
		return nil, errors.New("must set tlsconfig for wss")
	}

	return ml(s, address)
}

func tcpMakeListener(network string) MakeListener {
	return func(s *Server, address string) (ln net.Listener, err error) {
		if s.tlsConfig == nil {
			ln, err = net.Listen(network, address)
		} else {
			ln, err = tls.Listen(network, address, s.tlsConfig)
		}

		return ln, err
	}
}
