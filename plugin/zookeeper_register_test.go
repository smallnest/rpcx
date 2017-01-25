package plugin

import (
	"os"
	"testing"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/smallnest/rpcx/log"
)

func TestZooKeeperRegisterPlugin_Register(t *testing.T) {
	if os.Getenv("travis") != "" {
		log.Info("test in travis-ci.org and it has not installed zookeeper, so don't test this case")
		return
	}

	plugin := &ZooKeeperRegisterPlugin{
		ServiceAddress:   "tcp@127.0.0.1:1234",
		ZooKeeperServers: []string{"127.0.0.1:2181"},
		BasePath:         "/rpcx",
		Metrics:          metrics.NewRegistry(),
		Services:         make([]string, 1),
		UpdateInterval:   time.Minute,
	}

	err := plugin.Start()
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	plugin.Conn.Create(plugin.BasePath, []byte("services"), flags, acl)

	err = plugin.Register("ABC", "aService")

	if err == zk.ErrNoServer {
		t.Log("must start a zookeeper at 127.0.0.1:2181 for this test")
		return
	}
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	_, _, err = plugin.Conn.Get(plugin.BasePath + "/ABC/tcp@127.0.0.1:1234")
	if err != nil {
		t.Errorf("service has not been registered on zookeeper: %v", err)
	}

	plugin.Unregister("ABC")
	_, _, err = plugin.Conn.Get(plugin.BasePath + "/ABC")
	if err != nil {
		t.Error("service has not been registered on zookeeper.")
	}

}
