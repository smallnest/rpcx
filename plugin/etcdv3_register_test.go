package plugin

import (
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/rcrowley/go-metrics"
	"github.com/smallnest/rpcx/log"
)

func TestEtcdV3RegisterPlugin_Register(t *testing.T) {
	if os.Getenv("travis") != "" {
		log.Infof("test in travis-ci.org and it has not installed etcd, so don't test this case")
		return
	}

	plugin := &EtcdV3RegisterPlugin{
		ServiceAddress:    "tcp@127.0.0.1:1234",
		EtcdServers:       []string{"http://127.0.0.1:2379"},
		BasePath:          "/etcdv3",
		Metrics:           metrics.NewRegistry(),
		Services:          make([]string, 1),
		UpdateInterval:    5 * time.Second,
		UpdateIntervalNum: 5,
		DialTimeout:       3 * time.Second,
	}

	err := plugin.Start()
	if err != nil {
		t.Log("must start a default etcd for this test" + err.Error())
		return
	}

	err = plugin.Register("ABC", "aService")

	if err != nil {
		t.Errorf("can't register this service")
	}

	resp, err := plugin.KeysAPI.Get(context.TODO(), plugin.BasePath+"/ABC/tcp@127.0.0.1:1234")
	if err != nil || resp.Kvs == nil {
		t.Errorf("service has not been registered on etcd: %v", err)
	} else {
		t.Logf("Get %s", resp.Kvs)
	}

	plugin.Unregister("ABC")
	_, err = plugin.KeysAPI.Get(context.TODO(), plugin.BasePath+"/ABC/tcp@127.0.0.1:1234")
	if err != nil {
		t.Error("service has not been registered on etcd.")
	}
}
