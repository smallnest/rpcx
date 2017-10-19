package client

import (
	"context"
	"testing"
	"time"

	"github.com/smallnest/rpcx/server"
)

func TestXClient_IT(t *testing.T) {
	s := server.Server{}
	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:0")
	defer s.Close()
	time.Sleep(500 * time.Millisecond)

	addr := s.Address().String()

	d := NewPeer2PeerDiscovery("tcp@"+addr, "desc=a test service")
	xclient := NewXClient("Arith", "Mul", Failtry, RandomSelect, d, DefaultOption)

	defer xclient.Close()

	args := &Args{
		A: 10,
		B: 20,
	}

	reply := &Reply{}
	err := xclient.Call(context.Background(), args, reply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if reply.C != 200 {
		t.Fatalf("expect 200 but got %d", reply.C)
	}
}
