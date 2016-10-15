package plugin

import (
	"net"
	"net/rpc"
	"time"

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

// PreReadRequestHeader marks start time of calling service
func (plugin *MetricsPlugin) PreReadRequestHeader(r *rpc.Request) error {
	plugin.seqs[r.Seq] = time.Now().UnixNano()
	return nil
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

	t := plugin.seqs[r.Seq]
	if t > 0 {
		t = time.Now().UnixNano() - t
		if t < 30*time.Minute.Nanoseconds() { //it is impossible that calltime exceeds 30 minute
			//Historgram
			h := metrics.GetOrRegisterHistogram("service_"+r.ServiceMethod+"_CallTime", plugin.Registry,
				metrics.NewExpDecaySample(1028, 0.015))
			h.Update(t)
		}
	}

	return nil
}

// Name return name of this plugin.
func (plugin *MetricsPlugin) Name() string {
	return "MetricsPlugin"
}
