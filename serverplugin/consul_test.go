package serverplugin

import (
	"testing"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/server"
)

func TestConsulRegistry(t *testing.T) {
	s := server.NewServer()
	r := NewConsulRegisterPlugin(
		WithConsulServiceAddress("tcp@127.0.0.1:8972"),
		WithConsulServers([]string{"127.0.0.1:8500"}),
		WithConsulBasePath("/rpcx_test"),
		WithConsulMetrics(metrics.NewRegistry()),
		WithConsulUpdateInterval(time.Minute),
	)
	err := r.Start()
	if err != nil {
		return
	}
	s.Plugins.Add(r)

	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:8972")
	defer s.Close()

	if len(r.Services) != 1 {
		t.Fatal("failed to register services in consul")
	}

	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
}
