package main

import (
	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/codec"
)

type Hello int

func (t *Hello) Say(args *BenchmarkMessage, reply *BenchmarkMessage) error {
	args.Field1 = "OK"
	args.Field2 = 100
	*reply = *args
	return nil
}

func main() {
	server := rpcx.NewServer()
	server.ServerCodecFunc = codec.NewProtobufServerCodec
	server.RegisterName("Hello", new(Hello))
	server.Serve("tcp", "127.0.0.1:8972")
}
