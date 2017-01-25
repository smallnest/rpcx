package main

import (
	"crypto/sha1"

	"golang.org/x/crypto/pbkdf2"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/log"
	kcp "github.com/xtaci/kcp-go"
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

const cryptKey = "rpcx-key"
const cryptSalt = "rpcx-salt"

func main() {
	server := rpcx.NewServer()
	server.RegisterName("Arith", new(Arith))

	pass := pbkdf2.Key([]byte(cryptKey), []byte(cryptSalt), 4096, 32, sha1.New)
	bc, err := kcp.NewAESBlockCrypt(pass)

	ln, err := kcp.ListenWithOptions("127.0.0.1:8972", bc, 10, 3)
	// kcplistener := ln
	// kcplistener.SetReadBuffer(16 * 1024 * 1024)
	// kcplistener.SetWriteBuffer(16 * 1024 * 1024)
	// kcplistener.SetDSCP(46)

	if err != nil {
		log.Infof(err.Error())
		return
	}
	server.ServeListener(ln)
}
