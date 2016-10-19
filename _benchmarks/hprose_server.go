package main

import (
	"flag"

	"github.com/hprose/hprose-golang/rpc"
)

func say(in []byte) ([]byte, error) {
	args := &BenchmarkMessage{}
	args.Unmarshal(in)
	args.Field1 = "OK"
	args.Field2 = 100
	return args.Marshal()
}

var host = flag.String("s", "127.0.0.1:8972", "listened ip and port")

func main() {
	flag.Parse()
	server := rpc.NewTCPServer("tcp://" + *host)
	server.AddFunction("say", say, rpc.Options{Simple: true})
	server.Start()
}
