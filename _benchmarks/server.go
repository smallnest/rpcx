package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/codec"
)

type Hello int

func (t *Hello) Say(args *BenchmarkMessage, reply *BenchmarkMessage) error {
	args.Field1 = "OK"
	args.Field2 = 100
	*reply = *args
	return nil
}

var host = flag.String("s", "127.0.0.1:8972", "listened ip and port")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	server := rpcx.NewServer()
	server.ServerCodecFunc = codec.NewProtobufServerCodec
	server.RegisterName("Hello", new(Hello))
	server.Serve("tcp", *host)
}
