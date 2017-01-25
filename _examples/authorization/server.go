package main

import (
	"errors"

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

	fn := func(p *rpcx.AuthorizationAndServiceMethod) error {
		if p.Authorization != "0b79bab50daca910b000d4f1a2b675d604257e42" || p.Tag != "Bearer" {
			log.Infof("error: wrong Authorization: %s, %s", p.Authorization, p.Tag)
			return errors.New("Authorization failed ")
		}

		log.Infof("Authorization success: %+v", p)
		return nil
	}

	server.Auth(fn)

	server.RegisterName("Arith", new(Arith))
	server.Serve("tcp", "127.0.0.1:8972")
}
