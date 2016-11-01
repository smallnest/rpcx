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

const cryptKey = "rpcx-key"
const cryptSalt = "rpcx-salt"

func main() {
	pass := pbkdf2.Key([]byte(cryptKey), []byte(cryptSalt), 4096, 32, sha1.New)
	bc, err := kcp.NewAESBlockCrypt(pass)

	s := &rpcx.DirectClientSelector{Network: "kcp", Address: "127.0.0.1:8972"}
	client := rpcx.NewClient(s)
	client.Block = bc

	args := &Args{7, 8}
	var reply Reply
	err = client.Call("Arith.Mul", args, &reply)
	if err != nil {
		fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		fmt.Printf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
