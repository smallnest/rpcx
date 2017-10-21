package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
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

func TestHandleRequest(t *testing.T) {
	//use jsoncodec

	req := protocol.NewMessage()
	req.SetVersion(0)
	req.SetMessageType(protocol.Request)
	req.SetHeartbeat(false)
	req.SetOneway(false)
	req.SetCompressType(protocol.None)
	req.SetMessageStatusType(protocol.Normal)
	req.SetSerializeType(protocol.JSON)
	req.SetSeq(1234567890)

	req.ServicePath = "Arith"
	req.ServiceMethod = "Mul"

	argv := &Args{
		A: 10,
		B: 20,
	}

	data, err := json.Marshal(argv)
	if err != nil {
		t.Fatal(err)
	}

	req.Payload = data

	server := &Server{}
	server.RegisterName("Arith", new(Arith), "")
	res, err := server.handleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to hand request: %v", err)
	}

	if res.Payload == nil {
		t.Fatalf("expect reply but got %s", res.Payload)
	}

	reply := &Reply{}

	codec := share.Codecs[res.SerializeType()]
	if codec == nil {
		t.Fatalf("can not find codec %c", codec)
	}

	err = codec.Decode(res.Payload, reply)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if reply.C != 200 {
		t.Fatalf("expect 200 but got %d", reply.C)
	}
}
