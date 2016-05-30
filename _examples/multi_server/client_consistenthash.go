package main

import (
	"fmt"
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/clientselector"
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}
type Reply struct {
	C int `msg:"c"`
}

func main() {
	server1 := clientselector.ServerPair{Network: "tcp", Address: "127.0.0.1:8972"}
	server2 := clientselector.ServerPair{Network: "tcp", Address: "127.0.0.1:8973"}

	servers := []clientselector.ServerPair{server1, server2}

	s := clientselector.NewMultiClientSelector(servers, rpcx.ConsistentHash, 10*time.Second)

	client := rpcx.NewClient(s)
	for i := 0; i < 10000; i++ {
		callServer(client)
	}
	client.Close()
}

func callServer(client *rpcx.Client) {	
	client.FailMode = rpcx.Failover
	args := &Args{7, 8}
	var reply Reply
	err := client.Call("Arith.Mul", args, &reply)
	if err != nil {
		fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		fmt.Printf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	
}
