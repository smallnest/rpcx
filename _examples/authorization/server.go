package main

import (
	"fmt"

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

	fn := func(p *plugin.AuthorizationAndServiceMethod) error {
		if p.Authorization != "0b79bab50daca910b000d4f1a2b675d604257e42" && p.Tag != "Bearer" {
			fmt.Printf("error: wrong Authorization: %s, %s\n", p.Authorization, p.Tag)
		} else {
			fmt.Println("Authorization success")
		}
		return nil
	}

	p := &plugin.AuthorizationServerPlugin{AuthorizationFunc: fn}
	server.PluginContainer.Add(p)

	server.RegisterName("Arith", new(Arith))
	server.Serve("tcp", "127.0.0.1:8972")
}
