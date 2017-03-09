package main

import (
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/plugin"
)

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func main() {
	server := rpcx.NewServer()
	server.ServerCodecFunc = codec.NewGobServerCodec
	server.RegisterName("Arith", new(Arith))

	p := plugin.NewRateLimitingPlugin(time.Second, 1000)
	server.PluginContainer.Add(p)

	server.Serve("tcp", "127.0.0.1:8972")
}
