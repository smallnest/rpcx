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

const cryptKey = "rpcx-key"
const cryptSalt = "rpcx-salt"

func main() {
	pass := pbkdf2.Key([]byte(cryptKey), []byte(cryptSalt), 4096, 32, sha1.New)
	bc, _ := kcp.NewAESBlockCrypt(pass)

	s := &rpcx.DirectClientSelector{Network: "kcp", Address: "127.0.0.1:8972"}
	client := rpcx.NewClient(s)
	client.Block = bc

	args := &Args{7, 8}
	var reply Reply
	err := client.Call("Arith.Mul", args, &reply)
	if err != nil {
		log.Infof("error for Arith: %d*%d, %v", args.A, args.B, err)
	} else {
		log.Infof("Arith: %d*%d=%d", args.A, args.B, reply.C)
	}

	client.Close()
}
