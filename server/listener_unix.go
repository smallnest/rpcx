// +build !windows

package server

import (
	"net"

	reuseport "github.com/kavu/go_reuseport"
)

func init() {
	makeListeners["reuseport"] = reuseportMakeListener
	makeListeners["unix"] = unixMakeListener
}

func reuseportMakeListener(s *Server, address string) (ln net.Listener, err error) {
	var network string
	if validIP4(address) {
		network = "tcp4"
	} else {
		network = "tcp6"
	}

	return reuseport.NewReusablePortListener(network, address)
}

func unixMakeListener(s *Server, address string) (ln net.Listener, err error) {
	laddr, err := net.ResolveUnixAddr("unix", address)
	if err != nil {
		return nil, err
	}
	return net.ListenUnix("unix", laddr)
}
