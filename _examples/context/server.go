package main

import (
	"net"

	"github.com/saiser/rpcx/log"
	"github.com/saiser/rpcx"
)

type Args struct {
	A   int
	B   int
	ctx map[string]interface{}
}

type Reply struct {
	C int
}

func (a *Args) Value(key string) interface{} {
	if a.ctx != nil {
		return a.ctx[key]
	}
	return nil
}

func (a *Args) SetValue(key string, value interface{}) {
	if a.ctx == nil {
		a.ctx = make(map[string]interface{})
	}
	a.ctx[key] = value
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	conn := args.Value("conn").(net.Conn)
	log.Infof("Client IP: %s", conn.RemoteAddr().String())
	return nil
}

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))
	server.Serve("tcp", "127.0.0.1:8972")
}
