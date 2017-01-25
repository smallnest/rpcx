package main

import (
	"flag"
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/clientselector"
	"github.com/smallnest/rpcx/log"
)

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

var e = flag.String("e", "http://127.0.0.1:2379", "etcd URL")
var n = flag.String("n", "Arith", "Service Name")

func main() {
	flag.Parse()

	//basePath = "/rpcx/" + serviceName
	s := clientselector.NewEtcdV3ClientSelector([]string{*e}, "/rpcx/"+*n, time.Minute, rpcx.RandomSelect, time.Minute)
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply

	for i := 0; i < 1000; i++ {
		err := client.Call(*n+".Mul", args, &reply)
		if err != nil {
			log.Infof("error for "+*n+": %d*%d, %v", args.A, args.B, err)
		} else {
			log.Infof(*n+": %d*%d=%d", args.A, args.B, reply.C)
		}
	}

	client.Close()
}
