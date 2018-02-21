// +build utp

package server

import (
	"net"

	"github.com/anacrolix/utp"
)

func init() {
	makeListeners["utp"] = utpMakeListener
}

func utpMakeListener(s *Server, address string) (ln net.Listener, err error) {
	return utp.Listen(address)
}
