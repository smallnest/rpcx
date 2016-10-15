package plugin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/samuel/go-zookeeper/zk"
)

//ZooKeeperRegisterPlugin a register plugin which can register services into zookeeper for cluster
type ZooKeeperRegisterPlugin struct {
	ServiceAddress   string
	ZooKeeperServers []string
	BasePath         string
	Conn             *zk.Conn
	Metrics          metrics.Registry
	Services         []string
	UpdateInterval   time.Duration
}

// Start starts to connect zookeeper cluster
func (plugin *ZooKeeperRegisterPlugin) Start() (err error) {
	conn, _, err := zk.Connect(plugin.ZooKeeperServers, time.Second)
	plugin.Conn = conn

	if plugin.UpdateInterval > 0 {
		ticker := time.NewTicker(plugin.UpdateInterval)
		go func() {
			for range ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Metrics)
				data := []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
				//set this same metrics for all services at this server
				for _, name := range plugin.Services {
					nodePath := plugin.BasePath + "/" + name + "/" + plugin.ServiceAddress
					bytes, stat, err := conn.Get(nodePath)
					if err == nil {
						v, _ := url.ParseQuery(string(bytes))
						v.Set("tps", string(data))

						conn.Set(nodePath, []byte(v.Encode()), stat.Version)
					}

				}

			}
		}()
	}

	return
}

func updateServerInfo(conn *zk.Conn) {

}

// HandleConnAccept handles connections from clients
func (plugin *ZooKeeperRegisterPlugin) HandleConnAccept(net.Conn) bool {
	if plugin.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Metrics)
		clientMeter.Mark(1)
	}
	return true
}

//Close closes zookeeper connection.
func (plugin *ZooKeeperRegisterPlugin) Close() {
	plugin.Conn.Close()
}

func mkdirs(conn *zk.Conn, path string) (err error) {
	if path == "" {
		return errors.New("path should not been empty")
	}
	if path == "/" {
		return nil
	}
	if path[0] != '/' {
		return errors.New("path must start with /")
	}

	//check whether this path exists
	exist, _, err := conn.Exists(path)
	if exist {
		return nil
	}
	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	_, err = conn.Create(path, []byte(""), flags, acl)
	if err == nil { //created successfully
		return
	}

	//create parent
	paths := strings.Split(path[1:], "/")
	createdPath := ""
	for _, p := range paths {
		createdPath = createdPath + "/" + p
		exist, _, err = conn.Exists(createdPath)
		if !exist {
			_, err = conn.Create(createdPath, []byte(""), flags, acl)
			if err != nil {
				return
			}
		}
	}

	return nil
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (plugin *ZooKeeperRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) (err error) {
	nodePath := plugin.BasePath + "/" + name
	err = mkdirs(plugin.Conn, nodePath)
	if err != nil {
		return err
	}

	nodePath = nodePath + "/" + plugin.ServiceAddress
	//delete existed node
	exists, _, err := plugin.Conn.Exists(nodePath)
	if exists {
		err = plugin.Conn.Delete(nodePath, -1)
	}

	//create Ephemeral node
	flags := int32(zk.FlagEphemeral)
	acl := zk.WorldACL(zk.PermAll)
	_, err = plugin.Conn.Create(nodePath, []byte(strings.Join(metadata, "&")), flags, acl)

	plugin.Services = append(plugin.Services, name)
	return
}

// Unregister a service from zookeeper but this service still exists in this node.
func (plugin *ZooKeeperRegisterPlugin) Unregister(name string) {
	nodePath := plugin.BasePath + "/" + name + "/" + plugin.ServiceAddress

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
