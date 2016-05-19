package plugin

import (
	"net"
	"net/rpc"

	"github.com/rcrowley/go-metrics"
)

//MetricsPlugin collects metrics of a rpc server
type MetricsPlugin struct {
	Registry metrics.Registry

}

//NewMetricsPlugin creates a new MetricsPlugirn
func NewMetricsPlugin() *MetricsPlugin {
	return &MetricsPlugin{Registry: metrics.NewRegistry()}
}

// Register handles registering event.
func (plugin *MetricsPlugin) Register(name string, rcvr interface{}) error {
	serviceCounter := metrics.GetOrRegisterCounter("serviceCounter", plugin.Registry)
	serviceCounter.Inc(1)
	return nil
}

// Handle connections from clients
func (plugin *MetricsPlugin) Handle(net.Conn) bool {
	clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Registry)
	clientMeter.Mark(1)
	return true
}



// PostReadRequestHeader counts read
func (plugin *MetricsPlugin) PostReadRequestHeader(r *rpc.Request) {
	if r.ServiceMethod == "" {
		return
	}

	m := metrics.GetOrRegisterMeter("service_"+r.ServiceMethod+"_Read_Counter", plugin.Registry)
	m.Mark(1)
}

// PostWriteResponse count write
func (plugin *MetricsPlugin) PostWriteResponse(r *rpc.Response, body interface{}) {
	if r.ServiceMethod == "" {
		return
	}

	m := metrics.GetOrRegisterMeter("service_"+r.ServiceMethod+"_Write_Counter", plugin.Registry)
	m.Mark(1)
}

// Name return name of this plugin.
func (plugin *MetricsPlugin) Name() string {
	return "MetricsPlugin"
}

// Description return description of this plugin.
func (plugin *MetricsPlugin) Description() string {
	return "a register plugin which collects metrics of a rpc server"
}
