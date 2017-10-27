// +build !windows

package server

import (

	reuseport "github.com/kavu/go_reuseport"
)

func init() {
	makeListeners["reuseport"] = reuseportMakeListener
}


func reuseportMakeListener func(s *Server, address string) (ln net.Listener, err error) {
	if validIP4(address) {
		network = "tcp4"
	} else {
		network = "tcp6"
	}

	return reuseport.NewReusablePortListener(network, address)
}