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
	s := clientselector.NewEtcdClientSelector([]string{"http://127.0.0.1:2379"}, "/rpcx/Arith", time.Minute, rpcx.RandomSelect, time.Minute)
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply

	for i := 0; i < 1000; i++ {
		err := client.Call("Arith.Mul", args, &reply)
		if err != nil {
			fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
		} else {
			fmt.Printf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
		}
	}

	client.Close()
}
