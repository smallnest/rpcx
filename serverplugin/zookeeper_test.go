package serverplugin

import (
	"testing"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/server"
)

func TestZookeeperRegistry(t *testing.T) {
	s := server.NewServer()
	r := NewZooKeeperRegisterPlugin(
		WithZKServiceAddress("tcp@127.0.0.1:8972"),
		WithZKServersAddress([]string{"127.0.0.1:2181"}),
		WithZKBasePath("rpcx_test"),
		WithZKMetrics(metrics.NewRegistry()),
		WithZkUpdateInterval(time.Minute))
	err := r.Start()
	if err != nil {
		return
	}
	s.Plugins.Add(r)

	s.RegisterName("Arith", new(Arith), "")
	go s.Serve("tcp", "127.0.0.1:8972")
	defer s.Close()

	if len(r.Services) != 1 {
		t.Fatal("failed to register services in zookeeper")
	}

	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
}
