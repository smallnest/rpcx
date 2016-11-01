package main

import (
	"crypto/sha1"
	"fmt"

	"golang.org/x/crypto/pbkdf2"

	"github.com/smallnest/rpcx"
	kcp "github.com/xtaci/kcp-go"
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
		fmt.Println(err.Error())
		return
	}
	server.ServeListener(ln)
}
