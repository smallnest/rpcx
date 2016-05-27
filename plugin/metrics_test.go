package plugin

import (
	"testing"

	"github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx"
)

var (
	serviceName       = "Arith/1.0"
	serviceMethodName = "Arith/1.0.Mul"
	service           = new(Arith)
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

func TestMetrics(t *testing.T) {
	server := rpcx.NewServer()
	plugin := NewMetricsPlugin()
	server.PluginContainer.Add(plugin)

	server.RegisterName(serviceName, service)
	server.Start("tcp", "127.0.0.1:0")
	serverAddr := server.Address()

	s := &rpcx.DirectClientSelector{Network: "tcp", Address: serverAddr}
	client := rpcx.NewClient(s)

	args := &Args{7, 8}
	var reply Reply
	err := client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	}

	client.Close()
	server.Close()

	isOK := true
	plugin.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			isOK = isOK && metric.Count() == 1
		case metrics.Meter:
			isOK = isOK && metric.Count() == 1
		}
	})

	if !isOK {
		t.Fail()
	}
}
