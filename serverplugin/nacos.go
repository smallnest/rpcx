package serverplugin

import (
	"errors"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/smallnest/rpcx/v5/log"
	"github.com/smallnest/rpcx/v5/util"
)

// NacosRegisterPlugin implements consul registry.
type NacosRegisterPlugin struct {
	// service address, for example, tcp@127.0.0.1:8972, quic@127.0.0.1:1234
	ServiceAddress string
	// nacos client config
	ClientConfig constant.ClientConfig
	// nacos server config
	ServerConfig []constant.ServerConfig
	Cluster      string
	Tenant       string

	// Registered services
	Services []string

	namingClient naming_client.INamingClient

	dying chan struct{}
	done  chan struct{}
}

// Start starts to connect consul cluster
func (p *NacosRegisterPlugin) Start() error {
	if p.done == nil {
		p.done = make(chan struct{})
	}
	if p.dying == nil {
		p.dying = make(chan struct{})
	}

	namingClient, err := clients.CreateNamingClient(map[string]interface{}{
		"clientConfig":  p.ClientConfig,
		"serverConfigs": p.ServerConfig,
	})
	if err != nil {
		return err
	}

	p.namingClient = namingClient

	return nil
}

// Stop unregister all services.
func (p *NacosRegisterPlugin) Stop() error {
	_, ip, port, _ := util.ParseRpcxAddress(p.ServiceAddress)

	for _, name := range p.Services {

		inst := vo.DeregisterInstanceParam{
			Ip:          ip,
			Ephemeral:   true,
			Port:        uint64(port),
			ServiceName: name,
			Cluster:     p.Cluster,
		}
		_, err := p.namingClient.DeregisterInstance(inst)
		if err != nil {
			log.Errorf("faield to deregister %s: %v", name, err)
		}
	}

	close(p.dying)
	<-p.done

	return nil
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *NacosRegisterPlugin) Register(name string, rcvr interface{}, metadata string) (err error) {
	if strings.TrimSpace(name) == "" {
		return errors.New("Register service `name` can't be empty")
	}

	network, ip, port, err := util.ParseRpcxAddress(p.ServiceAddress)
	if err != nil {
		log.Errorf("failed to parse rpcx addr in Register: %v", err)
		return err
	}

	meta := util.ConvertMeta2Map(metadata)
	meta["network"] = network

	inst := vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: name,
		Metadata:    meta,
		ClusterName: p.Cluster,
		Tenant:      p.Tenant,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
	}

	_, err = p.namingClient.RegisterInstance(inst)
	if err != nil {
		log.Errorf("failed to register %s: %v", name, err)
		return err
	}

	p.Services = append(p.Services, name)

	return
}

func (p *NacosRegisterPlugin) RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	return p.Register(serviceName, fn, metadata)
}

func (p *NacosRegisterPlugin) Unregister(name string) (err error) {
	if strings.TrimSpace(name) == "" {
		return errors.New("Unregister service `name` can't be empty")
	}

	_, ip, port, err := util.ParseRpcxAddress(p.ServiceAddress)
	if err != nil {
		log.Errorf("wrong address %s: %v", p.ServiceAddress, err)
		return err
	}

	inst := vo.DeregisterInstanceParam{
		Ip:          ip,
		Ephemeral:   true,
		Port:        uint64(port),
		ServiceName: name,
		Cluster:     p.Cluster,
	}
	_, err = p.namingClient.DeregisterInstance(inst)
	if err != nil {
		log.Errorf("failed to deregister %s: %v", name, err)
		return err
	}

	var services = make([]string, 0, len(p.Services)-1)
	for _, s := range p.Services {
		if s != name {
			services = append(services, s)
		}
	}
	p.Services = services

	return nil
}
