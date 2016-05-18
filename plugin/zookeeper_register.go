package plugin

import (
	"fmt"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

//ZooKeeperRegisterPlugin a register plugin which can register services into zookeeper for cluster
type ZooKeeperRegisterPlugin struct {
	ServiceAddress   string
	ZooKeeperServers []string
	BasePath         string
	Conn             *zk.Conn
}

// Start starts to connect zookeeper cluster
func (plugin *ZooKeeperRegisterPlugin) Start() (err error) {
	conn, _, err := zk.Connect(plugin.ZooKeeperServers, time.Second)
	plugin.Conn = conn
	return
}

//Close closes zookeeper connection.
func (plugin *ZooKeeperRegisterPlugin) Close() {
	plugin.Conn.Close()
}

// Register handles registering event.
func (plugin *ZooKeeperRegisterPlugin) Register(name string, rcvr interface{}) (err error) {
	nodePath := plugin.BasePath + "/" + name

	//delete existed node
	exists, _, err := plugin.Conn.Exists(nodePath)
	if exists {
		err = plugin.Conn.Delete(nodePath, -1)
		fmt.Printf("delete: ok\n")
	}

	//create Ephemeral node
	//flags := int32(0)
	flags := int32(zk.FlagEphemeral)
	acl := zk.WorldACL(zk.PermAll)
	path, err := plugin.Conn.Create(nodePath, []byte(plugin.ServiceAddress), flags, acl)
	if err != nil {
		return
	}
	fmt.Printf("create: %+v\n", path)

	// stat, err = plugin.Conn.Set(nodePath, []byte(plugin.ServiceAddress), stat.Version)
	// data, stat, err := plugin.Conn.Get(nodePath)
	// if err != nil {

	// }
	// fmt.Printf("get:    %+v %+v\n", string(data), stat)

	return
}

// UnRegister a service from zookeeper but this service still exists in this node.
func (plugin *ZooKeeperRegisterPlugin) Unregister(name string) {
	nodePath := plugin.BasePath + "/" + name

	//delete existed node
	exists, _, _ := plugin.Conn.Exists(nodePath)
	if exists {
		err := plugin.Conn.Delete(nodePath, -1)
		if err != nil {
			fmt.Printf("delete: false because of %v\n", err)
		} else {
			fmt.Printf("delete: ok\n")
		}

	}
}

// Name return name of this plugin.
func (plugin *ZooKeeperRegisterPlugin) Name() string {
	return "ZooKeeperRegisterPlugin"
}

// Description return description of this plugin.
func (plugin *ZooKeeperRegisterPlugin) Description() string {
	return "a register plugin which can register services into zookeeper for cluster"
}
