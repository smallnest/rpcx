package main

import (
	"flag"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/plugin"
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

var addr = flag.String("s", "127.0.0.1:8972", "service address")
var e = flag.String("e", "http://127.0.0.1:2379", "etcd URL")
var n = flag.String("n", "Arith", "Service Name")

func main() {
	flag.Parse()

	server := rpcx.NewServer()
	rplugin := &plugin.EtcdRegisterPlugin{
		ServiceAddress: "tcp@" + *addr,
		EtcdServers:    []string{*e},
		BasePath:       "/rpcx",
		Metrics:        metrics.NewRegistry(),
		Services:       make([]string, 1),
		UpdateInterval: time.Minute,
	}
	rplugin.Start()
	server.PluginContainer.Add(rplugin)
	server.PluginContainer.Add(plugin.NewMetricsPlugin())
	server.RegisterName(*n, new(Arith), "weight=1&m=devops")
	server.Serve("tcp", *addr)
}
