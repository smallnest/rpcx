package main

import (
	"github.com/saiser/rpcx"
	"github.com/saiser/rpcx/plugin"
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

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))

	p := plugin.NewCompressionPlugin(rpcx.CompressSnappy)
	server.PluginContainer.Add(p)

	server.Serve("tcp", "127.0.0.1:8972")
}
