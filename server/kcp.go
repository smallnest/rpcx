// +build kcp

package server

import (
	"errors"
	"net"

	kcp "github.com/xtaci/kcp-go"
)

func init() {
	makeListeners["kcp"] = kcpMakeListener
}

func kcpMakeListener(s *Server, address string) (ln net.Listener, err error) {
	if s.options == nil || s.options["BlockCrypt"] == nil {
		return nil, errors.New("KCP BlockCrypt must be configured in server.Options")
	}

	return kcp.ListenWithOptions(address, s.options["BlockCrypt"].(kcp.BlockCrypt), 10, 3)
}

// WithBlockCrypt sets kcp.BlockCrypt.
func WithBlockCrypt(bc kcp.BlockCrypt) OptionFn {
	return func(s *Server) {
		s.options["BlockCrypt"] = bc
	}
}
