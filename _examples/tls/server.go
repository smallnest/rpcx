package main

import (
	"crypto/tls"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/log"
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
	server.RegisterName("Arith", new(Arith))

	cert, err := tls.LoadX509KeyPair("server.pem", "server.key")
	if err != nil {
		log.Info(err)
		return
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	server.ServeTLS("tcp", "127.0.0.1:8972", config)
}
