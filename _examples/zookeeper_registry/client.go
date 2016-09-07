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
	//basePath = "/rpcx/" + serviceName
	s := clientselector.NewZooKeeperClientSelector([]string{"127.0.0.1:2181"}, "/rpcx/Arith", 2*time.Minute, rpcx.WeightedRoundRobin, time.Minute)
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply

	for i := 0; i < 10; i++ {
		err := client.Call("Arith.Mul", args, &reply)
		if err != nil {
			fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
		} else {
			fmt.Printf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
		}
	}

	client.Close()
}
