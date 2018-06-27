package client

import (
	"context"
	"testing"
	"time"

	"github.com/smallnest/rpcx/server"
	"github.com/smallnest/rpcx/protocol"
)

func TestXClient_IT(t *testing.T) {
	s := server.NewServer()
	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:0")
	defer s.Close()
	time.Sleep(500 * time.Millisecond)

	addr := s.Address().String()

	d := NewPeer2PeerDiscovery("tcp@"+addr, "desc=a test service")
	xclient := NewXClient("Arith", Failtry, RandomSelect, d, DefaultOption)

	defer xclient.Close()

	args := &Args{
		A: 10,
		B: 20,
	}

	reply := &Reply{}
	err := xclient.Call(context.Background(), "Mul", args, reply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if reply.C != 200 {
		t.Fatalf("expect 200 but got %d", reply.C)
	}
}

func TestXClient_filterByStateAndGroup(t *testing.T) {
	servers := map[string]string{"a": "", "b": "state=inactive&ops=10", "c": "ops=20", "d": "group=test&ops=20"}
	filterByStateAndGroup("test", servers)
	if _, ok := servers["b"]; ok {
		t.Error("has not remove inactive node")
	}
	if _, ok := servers["a"]; ok {
		t.Error("has not remove inactive node")
	}
	if _, ok := servers["c"]; ok {
		t.Error("has not remove inactive node")
	}
	if _, ok := servers["d"]; !ok {
		t.Error("node must be removed")
	}
}


func TestDynamicCall(t *testing.T) {
	param := protocol.NewRpcParam()
	param.PutValue("a", 1)
	param.PutValue("b", 2)
	reply := protocol.RpcResult{}
	d := NewPeer2PeerDiscovery("tcp@127.0.0.1:8999", "desc=a test service")
	client := NewXClient("Arith", Failtry, RandomSelect, d, DefaultOption)
	client.Call(context.Background(), "DynamicMul", param, &reply)
	c := reply.Int64Value()
	if c != 3 {
		t.Fatalf("expect 3 but got %d", c)
	}
}