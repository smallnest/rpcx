package plugin

import (
	"testing"

	"github.com/samuel/go-zookeeper/zk"
)

func TestZooKeeperRegisterPlugin_Register(t *testing.T) {
	plugin := &ZooKeeperRegisterPlugin{
		ServiceAddress:   "tcp://127.0.0.1:1234",
		ZooKeeperServers: []string{"127.0.0.1"},
		BasePath:         "/betterrpc",
	}

	err := plugin.Start()
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	plugin.Conn.Create(plugin.BasePath, []byte("services"), flags, acl)

	err = plugin.Register("ABC", "aService")
	if err != nil {
		t.Errorf("can't start this plugin: %v", err)
	}

	data, _, err := plugin.Conn.Get(plugin.BasePath + "/ABC")
	if err != nil {
		t.Errorf("service has not been registered on zookeeper: %v", err)
	}
	if string(data) != plugin.ServiceAddress {
		t.Errorf("registered address of services is not right. Got: %v, Expected: %v", data, plugin.ServiceAddress)
	}

	plugin.Unregister("ABC")
	data, _, _ = plugin.Conn.Get(plugin.BasePath + "/ABC")
	if data != nil {
		t.Errorf("service has not been unregistered on zookeeper. Got: %v", string(data))
	}

}
