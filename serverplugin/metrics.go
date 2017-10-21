package serverplugin

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	"github.com/smallnest/rpcx/protocol"
	influxdb "github.com/vrischmann/go-metrics-influxdb"
)

// MetricsPlugin has an issue. It changes seq of requests and it is wrong!!!!
// we should use other methods to map requests and responses not but seq.

// MetricsPlugin collects metrics of a rpc server.
// You can report metrics to log, syslog, Graphite, InfluxDB or others to display them in Dashboard such as grafana, Graphite.
type MetricsPlugin struct {
	Registry   metrics.Registry
	Prefix     string
	timeSeqMap map[context.Context]int64
	mapLock    sync.RWMutex
}

//NewMetricsPlugin creates a new MetricsPlugirn
func NewMetricsPlugin() *MetricsPlugin {
	return &MetricsPlugin{Registry: metrics.NewRegistry(), timeSeqMap: make(map[context.Context]int64, 100)}
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
	p.mapLock.Lock()
	defer p.mapLock.Unlock()

	p.timeSeqMap[ctx] = time.Now().UnixNano()
	return nil
}

// PostReadRequest counts read
func (p *MetricsPlugin) PostReadRequest(ctx context.Context, r *protocol.Message, e error) error {
	sp := r.ServicePath
	sm := r.ServiceMethod

	m := metrics.GetOrRegisterMeter(p.withPrefix("service_"+sp+"."+sm+"_Read_Qps"), p.Registry)
	m.Mark(1)
	return nil
}

// PostWriteResponse count write
func (p *MetricsPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, e error) error {
	sp := res.ServicePath
	sm := res.ServiceMethod

	m := metrics.GetOrRegisterMeter(p.withPrefix("service_"+sp+"."+sm+"_Write_Qps"), p.Registry)
	m.Mark(1)

	p.mapLock.Lock()
	t := p.timeSeqMap[ctx]
	delete(p.timeSeqMap, ctx)
	p.mapLock.Unlock()

	if t > 0 {
		t = time.Now().UnixNano() - t
		if t < 30*time.Minute.Nanoseconds() { //it is impossible that calltime exceeds 30 minute
			//Historgram
			h := metrics.GetOrRegisterHistogram(p.withPrefix("service_"+sp+"."+sm+"_CallTime"), p.Registry,
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
// 	p.InfluxDB(10e9, "127.0.0.1:8086","metrics", "test","test"})
//
func (p *MetricsPlugin) InfluxDB(freq time.Duration, url, database, username, password string) {
	go influxdb.InfluxDB(p.Registry, freq, url, database, username, password)
}

// Exp uses the same mechanism as the official expvar but exposed under /debug/metrics,
// which shows a json representation of all your usual expvars as well as all your go-metrics.
func (p *MetricsPlugin) Exp() {
	exp.Exp(metrics.DefaultRegistry)
}
