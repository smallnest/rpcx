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

var zk = flag.String("zk", "127.0.0.1:2181", "zookeeper URL")
var n = flag.String("n", "127.0.0.1:2181", "Arith")

func main() {
	flag.Parse()

	//basePath = "/rpcx/" + serviceName
	s := clientselector.NewZooKeeperClientSelector([]string{*zk}, "/rpcx/"+*n, 2*time.Minute, rpcx.Closest, time.Minute)
	s.Latitude = 42.3159
	s.Longitude = -71.0559
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply

	for i := 0; i < 10; i++ {
		err := client.Call(*n+".Mul", args, &reply)
		if err != nil {
			log.Infof("error for "+*n+": %d*%d, %v", args.A, args.B, err)
		} else {
			log.Infof(*n+": %d*%d=%d", args.A, args.B, reply.C)
		}
	}

	client.Close()
}
