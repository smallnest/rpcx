package plugin

import (
	"strconv"
	"time"

	"strings"

	"github.com/hashicorp/consul/api"
)

//ConsulRegisterPlugin a register plugin which can register services into consul for cluster
//This registry is experimental and has not been test.
type ConsulRegisterPlugin struct {
	ServiceAddress string
	ConsulAddress  string
	consulConfig   *api.Config
	client         *api.Client
	Services       []string
	updateInterval time.Duration
}

// Start starts to connect etcd cluster
func (plugin *ConsulRegisterPlugin) Start() (err error) {
	if plugin.consulConfig == nil {
		plugin.consulConfig = api.DefaultConfig()
		plugin.consulConfig.Address = plugin.ConsulAddress
	}
	plugin.client, err = api.NewClient(plugin.consulConfig)

	if err != nil {
		return err
	}

	return
}

//Close closes this plugin
func (plugin *ConsulRegisterPlugin) Close() {

}

// Register handles registering event.
func (plugin *ConsulRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) (err error) {
	service := &api.AgentServiceRegistration{
		ID:      name + "-" + plugin.ServiceAddress,
		Name:    name,
		Address: plugin.ServiceAddress,
		Tags:    []string{strings.Join(metadata, "&")},
		Check: &api.AgentServiceCheck{
			TTL:    strconv.Itoa(int(plugin.updateInterval.Seconds())) + "s",
			Status: api.HealthPassing,
			TCP:    plugin.ServiceAddress,
		},
	}
	agent := plugin.client.Agent()
	err = agent.ServiceRegister(service)
	return
}

// Unregister a service from consul but this service still exists in this node.
func (plugin *ConsulRegisterPlugin) Unregister(name string) {
	agent := plugin.client.Agent()
	id := name + "-" + plugin.ServiceAddress
	agent.ServiceDeregister(id)
}

// CheckPass sets check pass
func (plugin *ConsulRegisterPlugin) CheckPass(name string) {
	agent := plugin.client.Agent()
	id := name + "-" + plugin.ServiceAddress
	agent.UpdateTTL("service:"+id, "", api.HealthPassing)
}

// CheckFail sets check fail
func (plugin *ConsulRegisterPlugin) CheckFail(name string) {
	agent := plugin.client.Agent()
	id := name + "-" + plugin.ServiceAddress
	agent.UpdateTTL("service:"+id, "", api.HealthCritical)
}

// FindServices gets a service list by name
func (plugin *ConsulRegisterPlugin) FindServices(name string) []*api.AgentService {
	agent := plugin.client.Agent()
	ass, err := agent.Services()
	if err != nil {
		return nil
	}

	var services []*api.AgentService
	for as, v := range ass {
		if strings.HasPrefix(as, name+"-") {
			services = append(services, v)
		}
	}

	return services
}

// Name return name of this plugin.
func (plugin *ConsulRegisterPlugin) Name() string {
	return "ConsulRegisterPlugin"
}
