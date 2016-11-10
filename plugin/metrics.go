package plugin

import (
	"net"
	"net/rpc"

	"github.com/rcrowley/go-metrics"
)

//MetricsPlugin collects metrics of a rpc server
type MetricsPlugin struct {
	Registry metrics.Registry
	seqs     map[uint64]int64
}

//NewMetricsPlugin creates a new MetricsPlugirn
func NewMetricsPlugin() *MetricsPlugin {
	return &MetricsPlugin{Registry: metrics.NewRegistry(), seqs: make(map[uint64]int64, 100)}
}

// Register handles registering event.
func (plugin *MetricsPlugin) Register(name string, rcvr interface{}, metadata ...string) error {
	serviceCounter := metrics.GetOrRegisterCounter("serviceCounter", plugin.Registry)
	serviceCounter.Inc(1)
	return nil
}

// HandleConnAccept handles connections from clients
func (plugin *MetricsPlugin) HandleConnAccept(net.Conn) bool {
	clientMeter := metrics.GetOrRegisterMeter("clientMeter", plugin.Registry)
	clientMeter.Mark(1)
	return true
}

// PostReadRequestHeader counts read
func (plugin *MetricsPlugin) PostReadRequestHeader(r *rpc.Request) error {
	if r.ServiceMethod == "" {
		return nil
	}

	m := metrics.GetOrRegisterMeter("service_"+r.ServiceMethod+"_Read_Counter", plugin.Registry)
	m.Mark(1)
	return nil
}

// PostWriteResponse count write
func (plugin *MetricsPlugin) PostWriteResponse(r *rpc.Response, body interface{}) error {
	if r.ServiceMethod == "" {
		return nil
	}

	m := metrics.GetOrRegisterMeter("service_"+r.ServiceMethod+"_Write_Counter", plugin.Registry)
	m.Mark(1)

	return nil
}

// Name return name of this plugin.
func (plugin *MetricsPlugin) Name() string {
	return "MetricsPlugin"
}
