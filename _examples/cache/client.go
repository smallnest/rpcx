package main

import (
	"fmt"
	"sync"
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
	server1 := clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8972"}
	server2 := clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8973"}

	servers := []clientselector.ServerPeer{server1, server2}

	s := clientselector.NewMultiClientSelector(servers, rpcx.RandomSelect, 10*time.Second)

	clientPool := &sync.Pool{
		New: func() interface{} {
			return rpcx.NewClient(s)
		},
	}
	for i := 0; i < 10000; i++ {
		callServer(clientPool, s)
	}
}

func callServer(clientPool *sync.Pool, s rpcx.ClientSelector) {
	client := clientPool.Get().(*rpcx.Client)

	args := &Args{7, 8}
	var reply Reply
	err := client.Call("Arith.Mul", args, &reply)
	if err != nil {
		fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		fmt.Printf("Arith: %d*%d=%d, client: %p \n", args.A, args.B, reply.C, client)
	}

	clientPool.Put(client)
}
