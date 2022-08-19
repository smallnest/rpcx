package server

import (
	"net"

	"github.com/akutz/memconn"
)

func init() {
	makeListeners["memu"] = memconnMakeListener
}

func memconnMakeListener(s *Server, address string) (ln net.Listener, err error) {
	return memconn.Listen("memu", address)
}
