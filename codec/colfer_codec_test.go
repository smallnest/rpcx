package codec

import (
	"testing"

	"github.com/smallnest/rpcx"
)

//go:generate colf go colfer_codec_test.colf

type ColfArith int

func (t *ColfArith) Mul(args *ColfArgs, reply *ColfReply) error {
	reply.C = args.A * args.B
	return nil
}

func TestColferCodec(t *testing.T) {
	server := rpcx.NewServer()
	server.ServerCodecFunc = NewColferServerCodec
	server.RegisterName(serviceName, new(ColfArith))
	server.Start("tcp", "127.0.0.1:0")
	serverAddr := server.Address()

	s := &rpcx.DirectClientSelector{Network: "tcp", Address: serverAddr}
	client := rpcx.NewClient(s)
	client.ClientCodecFunc = NewColferClientCodec

	args := &ColfArgs{7, 8}
	var reply ColfReply
	err := client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
