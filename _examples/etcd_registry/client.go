package main

import (
	"flag"
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

var e = flag.String("e", "http://127.0.0.1:2379", "etcd URL")
var n = flag.String("n", "Arith", "Service Name")

func main() {
	flag.Parse()

	//basePath = "/rpcx/" + serviceName
	s := clientselector.NewEtcdClientSelector([]string{*e}, "/rpcx/"+*n, time.Minute, rpcx.RandomSelect, time.Minute)
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply

	for i := 0; i < 1000; i++ {
		err := client.Call(*n+".Mul", args, &reply)
		if err != nil {
			fmt.Printf("error for "+*n+": %d*%d, %v \n", args.A, args.B, err)
		} else {
			fmt.Printf(*n+": %d*%d=%d \n", args.A, args.B, reply.C)
		}
	}

	client.Close()
}
