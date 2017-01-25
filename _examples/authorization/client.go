package main

import (
	"time"

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

func main() {
	s := &rpcx.DirectClientSelector{Network: "tcp", Address: "127.0.0.1:8972", DialTimeout: 10 * time.Second}
	client := rpcx.NewClient(s)

	//add Authorization info
	err := client.Auth("0b79bab50daca910b000d4f1a2b675d604257e42", "Bearer")
	if err != nil {
		log.Infof("can't add auth plugin: %#v", err)
	}

	args := &Args{7, 8}
	var reply Reply
	err = client.Call("Arith.Mul", args, &reply)
	if err != nil {
		log.Infof("error for Arith: %d*%d, %v", args.A, args.B, err)
	} else {
		log.Infof("Arith: %d*%d=%d", args.A, args.B, reply.C)
	}

	client.Close()
}
