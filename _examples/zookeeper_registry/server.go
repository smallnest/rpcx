package main

import (
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

func main() {
	server := rpcx.NewServer()
	plugin := &plugin.ZooKeeperRegisterPlugin{
		ServiceAddress:   "tcp@127.0.0.1:8972",
		ZooKeeperServers: []string{"127.0.0.1:2181"},
		BasePath:         "/rpcx",
		Metrics:          metrics.NewRegistry(),
		Services:         make([]string, 1),
		UpdateInterval:   time.Minute,
	}
	plugin.Start()
	server.PluginContainer.Add(plugin)
	server.RegisterName("Arith", new(Arith), "weight=5&state=active")
	server.Serve("tcp", "127.0.0.1:8972")
}
