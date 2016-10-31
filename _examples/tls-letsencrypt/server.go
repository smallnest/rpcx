package main

import "github.com/smallnest/rpcx"

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

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))

	server.ServeAutoTLS("tcp", "tr.colobu.com:443")
}
