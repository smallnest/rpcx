package serverplugin

import (
	"context"
	"net"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/server"
)

// MetricsPlugin has an issue. It changes seq of requests and it is wrong!!!!
// we should use other methods to map requests and responses not but seq.

// MetricsPlugin collects metrics of a rpc server.
// You can report metrics to log, syslog, Graphite, InfluxDB or others to display them in Dashboard such as grafana, Graphite.
type MetricsPlugin struct {
	Registry metrics.Registry
	Prefix   string
}

//NewMetricsPlugin creates a new MetricsPlugirn
func NewMetricsPlugin(registry metrics.Registry) *MetricsPlugin {
	return &MetricsPlugin{Registry: registry}
}

func (p *MetricsPlugin) withPrefix(m string) string {
	return p.Prefix + m
}

// Register handles registering event.
func (p *MetricsPlugin) Register(name string, rcvr interface{}, metadata string) error {
	serviceCounter := metrics.GetOrRegisterCounter(p.withPrefix("serviceCounter"), p.Registry)
	serviceCounter.Inc(1)
	return nil
}

// HandleConnAccept handles connections from clients
func (p *MetricsPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	clientMeter := metrics.GetOrRegisterMeter(p.withPrefix("clientMeter"), p.Registry)
	clientMeter.Mark(1)
	return conn, true
}

// PreReadRequest marks start time of calling service
func (p *MetricsPlugin) PreReadRequest(ctx context.Context) error {
	return nil
}

// PostReadRequest counts read
func (p *MetricsPlugin) PostReadRequest(ctx context.Context, r *protocol.Message, e error) error {
	sp := r.ServicePath
	sm := r.ServiceMethod

	if sp == "" {
		return nil
	}
	m := metrics.GetOrRegisterMeter(p.withPrefix("service."+sp+"."+sm+".Read_Qps"), p.Registry)
	m.Mark(1)
	return nil
}

// PostWriteResponse count write
func (p *MetricsPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, e error) error {
	sp := res.ServicePath
	sm := res.ServiceMethod

	if sp == "" {
		return nil
	}

	m := metrics.GetOrRegisterMeter(p.withPrefix("service."+sp+"."+sm+".Write_Qps"), p.Registry)
	m.Mark(1)

	t := ctx.Value(server.StartRequestContextKey).(int64)

	if t > 0 {
		t = time.Now().UnixNano() - t
		if t < 30*time.Minute.Nanoseconds() { //it is impossible that calltime exceeds 30 minute
			//Historgram
			h := metrics.GetOrRegisterHistogram(p.withPrefix("service."+sp+"."+sm+".CallTime"), p.Registry,
				metrics.NewExpDecaySample(1028, 0.015))
			h.Update(t)
		}
	}
	return nil
}

// Log reports metrics into logs.
//
// p.Log( 5 * time.Second, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
//
func (p *MetricsPlugin) Log(freq time.Duration, l metrics.Logger) {
	go metrics.Log(p.Registry, freq, l)
}

// Graphite reports metrics into graphite.
//
// 	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:2003")
//  p.Graphite(10e9, "metrics", addr)
//
func (p *MetricsPlugin) Graphite(freq time.Duration, prefix string, addr *net.TCPAddr) {
	go metrics.Graphite(p.Registry, freq, prefix, addr)
}

// InfluxDB reports metrics into influxdb.
//
// 	p.InfluxDB(10e9, "http://127.0.0.1:8086","metrics", "test","test"})
//
func (p *MetricsPlugin) InfluxDB(freq time.Duration, url, database, username, password string) {
	go InfluxDB(p.Registry, freq, url, database, username, password)
}

// InfluxDBWithTags reports metrics into influxdb with tags.
// you can set node info into tags.
//
// 	p.InfluxDBWithTags(10e9, "http://127.0.0.1:8086","metrics", "test","test", map[string]string{"host":"127.0.0.1"})
//
func (p *MetricsPlugin) InfluxDBWithTags(freq time.Duration, url, database, username, password string, tags map[string]string) {
	go InfluxDBWithTags(p.Registry, freq, url, database, username, password, tags)
}

// Exp uses the same mechanism as the official expvar but exposed under /debug/metrics,
// which shows a json representation of all your usual expvars as well as all your go-metrics.
func (p *MetricsPlugin) Exp() {
	exp.Exp(metrics.DefaultRegistry)
}
