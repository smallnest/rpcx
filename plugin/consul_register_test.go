package plugin

import (
	"os"
	"testing"
	"time"

	"github.com/saiser/rpcx/log"
)

func TestConsulRegisterPlugin_Register(t *testing.T) {
	if os.Getenv("travis") != "" {
		log.Infof("test in travis-ci.org and it has not installed consul, so don't test this case")
		return
	}

	plugin := &ConsulRegisterPlugin{
		ServiceAddress: "tcp@127.0.0.1:1234",
		ConsulAddress:  "localhost:8500",
		Services:       make([]string, 1),
		UpdateInterval: time.Second,
	}

	err := plugin.Start()
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	err = plugin.Register("ABC", "aService")
	if err != nil {
		t.Log("must start a default consul for this test")
		return
	}

	ss := plugin.FindServices("ABC")
	if ss == nil || len(ss) != 1 {
		t.Errorf("Service has not been registered")
	}
}
