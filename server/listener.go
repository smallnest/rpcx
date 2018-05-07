package server

import (
	"crypto/tls"
	"fmt"
	"net"
)

var makeListeners = make(map[string]MakeListener)

func init() {
	makeListeners["tcp"] = tcpMakeListener
	makeListeners["tcp4"] = tcp4MakeListener
	makeListeners["tcp6"] = tcp6MakeListener
	makeListeners["http"] = tcpMakeListener
}

// RegisterMakeListener registers a MakeListener for network.
func RegisterMakeListener(network string, ml MakeListener) {
	makeListeners[network] = ml
}

// MakeListener defines a listener generater.
type MakeListener func(s *Server, address string) (ln net.Listener, err error)

// block can be nil if the caller wishes to skip encryption in kcp.
// tlsConfig can be nil iff we are not using network "quic".
func (s *Server) makeListener(network, address string) (ln net.Listener, err error) {
	ml := makeListeners[network]
	if ml == nil {
		return nil, fmt.Errorf("can not make listener for %s", network)
	}
	return ml(s, address)
}

func tcpMakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.tlsConfig == nil {
		ln, err = net.Listen("tcp", address)
	} else {
		ln, err = tls.Listen("tcp", address, s.tlsConfig)
	}

	return ln, err
}

func tcp4MakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.tlsConfig == nil {
		ln, err = net.Listen("tcp4", address)
	} else {
		ln, err = tls.Listen("tcp4", address, s.tlsConfig)
	}

	return ln, err
}

func tcp6MakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.tlsConfig == nil {
		ln, err = net.Listen("tcp6", address)
	} else {
		ln, err = tls.Listen("tcp6", address, s.tlsConfig)
	}

	return ln, err
}
