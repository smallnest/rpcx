package plugin

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/rcrowley/go-metrics"
)

func TestEtcdRegisterPlugin_Register(t *testing.T) {
	plugin := &EtcdRegisterPlugin{
		ServiceAddress: "tcp@127.0.0.1:1234",
		EtcdServers:    []string{"http://127.0.0.1:2379"},
		BasePath:       "/rpcx",
		Metrics:        metrics.NewRegistry(),
		Services:       make([]string, 1),
		UpdateInterval: time.Minute,
	}

	err := plugin.Start()
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	err = plugin.Register("ABC", "aService")

	if err != nil {
		t.Log("must start a default etcd for this test")
		return
	}

	plugin.KeysAPI.Get(context.TODO(), plugin.BasePath+"/ABC/tcp@127.0.0.1:1234", nil)

	resp, err := plugin.KeysAPI.Get(context.TODO(), plugin.BasePath+"/ABC/tcp@127.0.0.1:1234", nil)
	if err != nil || resp.Node == nil {
		t.Errorf("service has not been registered on etcd: %v", err)
	}

	plugin.Unregister("ABC")
	resp, err = plugin.KeysAPI.Get(context.TODO(), plugin.BasePath+"/ABC/tcp@127.0.0.1:1234", nil)
	if err == nil {
		t.Error("service has not been registered on etcd.")
	}

}
