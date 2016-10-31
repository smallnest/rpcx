package main

import (
	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/plugin"
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

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))

	// p := server.PluginContainer.GetByName("AliasPlugin")
	// if p == nil {
	// 	p := plugin.NewAliasPlugin()
	// 	server.PluginContainer.Add(p)
	// }

	p := plugin.NewAliasPlugin()
	server.PluginContainer.Add(p)

	//set alias
	p.Alias("mul", "Arith.Mul")

	server.Serve("tcp", "127.0.0.1:8972")
}
