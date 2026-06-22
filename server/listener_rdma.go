//go:build rdma
// +build rdma

package server

import (
	"net"

	"github.com/smallnest/gordma/rdmanet"
	"github.com/smallnest/rpcx/share"
)

func init() {
	makeListeners["rdma"] = rdmaMakeListener
}

func rdmaMakeListener(s *Server, address string) (ln net.Listener, err error) {
	// Validate and normalize the host:port form rdmanet uses for its TCP
	// out-of-band handshake address. rdmanet.Listen exposes no backlog knob,
	// so the former RDMA_BACKLOG env var no longer applies.
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	l, err := rdmanet.Listen(net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}
	return share.NewRDMAListener(l), nil
}
