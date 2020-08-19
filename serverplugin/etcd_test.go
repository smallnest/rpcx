package serverplugin

import (
	"testing"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/v5/server"
)

func TestEtcdRegistry(t *testing.T) {
	s := server.NewServer()

	r := &EtcdRegisterPlugin{
		ServiceAddress: "tcp@127.0.0.1:8972",
		EtcdServers:    []string{"127.0.0.1:2379"},
		BasePath:       "/rpcx_test",
		Metrics:        metrics.NewRegistry(),
		UpdateInterval: time.Minute,
	}
	err := r.Start()
	if err != nil {
		return
	}
	s.Plugins.Add(r)

	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:8972")
	defer s.Close()

	if len(r.Services) != 1 {
		t.Fatal("failed to register services in etcd")
	}

	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
}
