package client

import (
	"context"
	"testing"
	"time"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
)

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

type Arith int

func (t *Arith) Mul(ctx context.Context, args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

type PBArith int

func (t *PBArith) Mul(ctx context.Context, args *ProtoArgs, reply *ProtoReply) error {
	reply.C = args.A * args.B
	return nil
}

func TestClient_IT(t *testing.T) {
	server := server.Server{}
	server.RegisterName("Arith", new(Arith))
	server.RegisterName("PBArith", new(PBArith))
	go server.Serve("tcp", "127.0.0.1:0")
	defer server.Close()
	time.Sleep(500 * time.Millisecond)

	addr := server.Address().String()

	client := &Client{
		SerializeType: protocol.JSON,
		CompressType:  protocol.Gzip,
	}

	err := client.Connect("tcp", addr)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	args := &Args{
		A: 10,
		B: 20,
	}

	reply := &Reply{}
	err = client.Call(context.Background(), "Arith", "Mul", args, reply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if reply.C != 200 {
		t.Fatalf("expect 200 but got %d", reply.C)
	}

	err = client.Call(context.Background(), "Arith", "Add", args, reply)
	if err == nil {
		t.Fatal("expect an error but got nil")
	}

	client.SerializeType = protocol.MsgPack
	reply = &Reply{}
	err = client.Call(context.Background(), "Arith", "Mul", args, reply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if reply.C != 200 {
		t.Fatalf("expect 200 but got %d", reply.C)
	}

	client.SerializeType = protocol.ProtoBuffer

	pbArgs := &ProtoArgs{
		A: 10,
		B: 20,
	}
	pbReply := &ProtoReply{}
	err = client.Call(context.Background(), "PBArith", "Mul", pbArgs, pbReply)
	if err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if pbReply.C != 200 {
		t.Fatalf("expect 200 but got %d", pbReply.C)
	}
}
