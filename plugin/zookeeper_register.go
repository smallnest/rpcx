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
	"github.com/smallnest/rpcx/log"
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
	if plugin.Conn, _, err = zk.Connect(plugin.ZooKeeperServers, time.Second); err != nil {
		return
	}

	if plugin.UpdateInterval > 0 {
		ticker := time.NewTicker(plugin.UpdateInterval)
		go func() {
			for range ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Metrics)
				data := []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
				//set this same metrics for all services at this server
				for _, name := range plugin.Services {
					nodePath := fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)
					bytes, stat, err := plugin.Conn.Get(nodePath)
					if err != nil {
						log.Infof("can't get data of node: %s, because of %v", nodePath, err.Error())
					} else {
						v, _ := url.ParseQuery(string(bytes))
						v.Set("tps", string(data))

						plugin.Conn.Set(nodePath, []byte(v.Encode()), stat.Version)
					}

				}

			}
		}()
	}

	return
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
	exist, _, _ := conn.Exists(path)
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
		exist, _, _ = conn.Exists(createdPath)
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
	if "" == strings.TrimSpace(name) {
		err = errors.New("Register service `name` can't be empty")
		return
	}
	nodePath := fmt.Sprintf("%s/%s", plugin.BasePath, name)
	if err = mkdirs(plugin.Conn, nodePath); err != nil {
		err = fmt.Errorf("can't create path: %s because of %v", nodePath, err)
		return
	}

	nodePath = fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)
	//delete existed node
	var exists bool
	if exists, _, err = plugin.Conn.Exists(nodePath); err != nil {
		err = fmt.Errorf("can't check path: %s because of %v", nodePath, err)
		return
	}
	if exists {
		if err = plugin.Conn.Delete(nodePath, -1); err != nil {
			err = fmt.Errorf("can't delete path: %s because of %v", nodePath, err)
			return
		}
	}

	//create Ephemeral node
	flags := int32(zk.FlagEphemeral)
	acl := zk.WorldACL(zk.PermAll)
	_, err = plugin.Conn.Create(nodePath, []byte(strings.Join(metadata, "&")), flags, acl)
	if err != nil {
		err = fmt.Errorf("can't create path: %s because of %v", nodePath, err)
		return
	}

	plugin.Services = append(plugin.Services, name)
	return
}

// Unregister a service from zookeeper but this service still exists in this node.
func (plugin *ZooKeeperRegisterPlugin) Unregister(name string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("Unregister service `name` can't be empty!")
		return
	}
	nodePath := fmt.Sprintf("%s/%s/%s", plugin.BasePath, name, plugin.ServiceAddress)

	//delete existed node
	var exists bool
	if exists, _, err = plugin.Conn.Exists(nodePath); err != nil {
		return
	}
	if exists {
		err = plugin.Conn.Delete(nodePath, -1)
		if err != nil {
			err = errors.New("delete: false because of " + err.Error())
			return
		}

		log.Info("delete: ok")

	}

	// because plugin.Start() method will be executed by timer continuously
	// so it need to remove the service name from service list
	if plugin.Services == nil || len(plugin.Services) <= 0 {
		return nil
	}
	var index int
	for index = 0; index < len(plugin.Services); index++ {
		if plugin.Services[index] == name {
			break
		}
	}
	if index != len(plugin.Services) {
		plugin.Services = append(plugin.Services[:index], plugin.Services[index+1:]...)
	}
	return
}

// Name return name of this plugin.
func (plugin *ZooKeeperRegisterPlugin) Name() string {
	return "ZooKeeperRegisterPlugin"
}
