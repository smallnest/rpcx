package codec

import (
	"sync"
	"testing"

	"github.com/smallnest/rpcx"
)

var (
	serverAddr        string
	serviceName       = "Arith/1.0"
	serviceMethodName = "Arith/1.0.Mul"
	service           = new(Arith)
	once              sync.Once
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func TestGobCodec(t *testing.T) {
	server := rpcx.NewServer()
	server.ServerCodecFunc = NewGobServerCodec
	server.RegisterName(serviceName, service)
	server.Start("tcp", "127.0.0.1:0")
	serverAddr := server.Address()

	s := &rpcx.DirectClientSelector{Network: "tcp", Address: serverAddr}
	client := rpcx.NewClient(s)
	client.ClientCodecFunc = NewGobClientCodec

	args := &Args{7, 8}
	var reply Reply
	err := client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	}

	client.Close()
}
