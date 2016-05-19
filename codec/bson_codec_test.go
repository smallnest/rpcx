package codec

import (
	"testing"

	"github.com/smallnest/betterrpc"
)

func TestBsonCodec(t *testing.T) {
	server := betterrpc.NewServer()
	server.ServerCodecFunc = NewBsonServerCodec
	server.RegisterName(serviceName, service)
	server.Start("tcp", "127.0.0.1:0")
	serverAddr := server.Address()

	s := &betterrpc.DirectClientSelector{Network: "tcp", Address: serverAddr}
	client := betterrpc.NewClient(s)
	client.ClientCodecFunc = NewBsonClientCodec
	err := client.Start()
	if err != nil {
		t.Errorf("can't connect to %s because of %v \n", serverAddr, err)
	}

	args := &Args{7, 8}
	var reply Reply
	err = client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
