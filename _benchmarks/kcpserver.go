package main

import (
	"flag"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/plugin"
)

type Hello int

func (t *Hello) Say(args *BenchmarkMessage, reply *BenchmarkMessage) error {
	args.Field1 = "OK"
	args.Field2 = 100
	*reply = *args
	return nil
}

var host = flag.String("s", "127.0.0.1:8972", "listened ip and port")

func main() {
	flag.Parse()
	server := rpcx.NewServer()
	p := plugin.NewCompressionPlugin(rpcx.CompressSnappy)
	server.PluginContainer.Add(p)

	server.ServerCodecFunc = codec.NewProtobufServerCodec
	server.RegisterName("Hello", new(Hello))
	server.Serve("kcp", *host)
}
