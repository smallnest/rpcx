//go:build rdma
// +build rdma

package server

import (
	"net"
	"os"
	"strconv"

	"github.com/smallnest/rsocket"
)

func init() {
	makeListeners["rdma"] = rdmaMakeListener
}

func rdmaMakeListener(s *Server, address string) (ln net.Listener, err error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	backlog := os.Getenv("RDMA_BACKLOG")
	if backlog == "" {
		backlog = "128"
	}
	blog, _ := strconv.Atoi(backlog)
	if blog == 0 {
		blog = 128
	}
	return rsocket.NewTCPListener(host, p, blog)
}
