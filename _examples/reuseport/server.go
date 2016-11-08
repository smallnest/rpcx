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

// start multiple instances and shutdown some instances in case of testing
func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))
	server.Serve("reuseport", ":8972")
}
