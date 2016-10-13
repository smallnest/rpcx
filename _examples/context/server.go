package main

import (
	"fmt"
	"net"

	"github.com/smallnest/rpcx"
)

type Args struct {
	A    int `msg:"a"`
	B    int `msg:"b"`
	conn net.Conn
}

type Reply struct {
	C int `msg:"c"`
}

func (a *Args) Conn() net.Conn {
	return a.conn
}

func (a *Args) SetConn(c net.Conn) {
	a.conn = c
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	fmt.Printf("Client IP: %s \n", args.Conn().RemoteAddr().String())
	return nil
}

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))
	server.Serve("tcp", "127.0.0.1:8972")
}
