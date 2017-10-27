// +build !windows

package server

import (
	"net"

	reuseport "github.com/kavu/go_reuseport"
)

func init() {
	makeListeners["reuseport"] = reuseportMakeListener
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
