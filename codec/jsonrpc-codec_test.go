package codec

import (
	"testing"

	"github.com/saiser/rpcx"
)

func TestJSONRPCCodec(t *testing.T) {
	server := rpcx.NewServer()
	server.ServerCodecFunc = NewJSONRPCServerCodec
	server.RegisterName(serviceName, service)
	server.Start("tcp", "127.0.0.1:0")
	serverAddr := server.Address()

	s := &rpcx.DirectClientSelector{Network: "tcp", Address: serverAddr}
	client := rpcx.NewClient(s)
	client.ClientCodecFunc = NewJSONRPCClientCodec

	args := &Args{7, 8}
	var reply Reply
	err := client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
