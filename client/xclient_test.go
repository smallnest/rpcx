package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"fmt"

	testutils "github.com/smallnest/rpcx/v5/_testutils"
	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/server"
	"github.com/smallnest/rpcx/v5/share"
)

func TestXClient_Thrift(t *testing.T) {
	s := server.NewServer()
	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:0")
	defer s.Close()
	time.Sleep(500 * time.Millisecond)

	addr := s.Address().String()

	opt := Option{
		Retries:        1,
		RPCPath:        share.DefaultRPCPath,
		ConnectTimeout: 10 * time.Second,
		SerializeType:  protocol.Thrift,
		CompressType:   protocol.None,
		BackupLatency:  10 * time.Millisecond,
	}

	d := NewPeer2PeerDiscovery("tcp@"+addr, "desc=a test service")
	xclient := NewXClient("Arith", Failtry, RandomSelect, d, opt)

	defer xclient.Close()

	args := testutils.ThriftArgs_{}
	args.A = 200
	args.B = 100

	reply := testutils.ThriftReply{}

	err := xclient.Call(context.Background(), "ThriftMul", &args, &reply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	fmt.Println(reply.C)
	if reply.C != 20000 {
		t.Fatalf("expect 20000 but got %d", reply.C)
	}
}

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

func TestUncoverError(t *testing.T) {
	var e error = ServiceError("error")
	if uncoverError(e) {
		t.Fatalf("expect false but get true")
	}

	if uncoverError(context.DeadlineExceeded) {
		t.Fatalf("expect false but get true")
	}

	if uncoverError(context.Canceled) {
		t.Fatalf("expect false but get true")
	}

	e = errors.New("error")
	if !uncoverError(e) {
		t.Fatalf("expect true but get false")
	}
}
