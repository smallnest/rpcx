package main

import (
	"time"

	"github.com/saiser/rpcx"
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
	server.Timeout = 1 * time.Nanosecond
	server.ReadTimeout = 1 * time.Nanosecond
	server.WriteTimeout = 1 * time.Nanosecond

	server.RegisterName("Arith", new(Arith))
	server.Serve("tcp", "127.0.0.1:8972")
}
