package main

import (
	"fmt"
	"time"

	"github.com/smallnest/rpcx"
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

func main() {
	s := &rpcx.DirectClientSelector{Network: "tcp", Address: "127.0.0.1:8972", DialTimeout: 10 * time.Second}
	client := rpcx.NewClient(s)

	//add Authorization info
	err := client.Auth("0b79bab50daca910b000d4f1a2b675d604257e42", "Bearer")
	if err != nil {
		fmt.Printf("can't add auth plugin: %#v\n", err)
	}

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
