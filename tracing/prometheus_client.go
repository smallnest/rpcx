package tracing

import (
	"context"
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
)

var (
	clientStartedCounter = prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "rpcx",
			Subsystem: "client",
			Name:      "started_total",
			Help:      "Total number of RPCs started on the client.",
		}, []string{"rpcx_method"})

	clientHandledCounter = prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "rpcx",
			Subsystem: "client",
			Name:      "handled_total",
			Help:      "Total number of RPCs completed by the client, regardless of success or failure.",
		}, []string{"rpcx_method"})
)

func init() {
	prom.MustRegister(clientStartedCounter)
	prom.MustRegister(clientHandledCounter)
}

type clientReporter struct {
	methodName string
	startTime  time.Time
}

func newClientReporter(method string) *clientReporter {
	r := &clientReporter{}
	r.methodName = method
	clientStartedCounter.WithLabelValues(r.methodName).Inc()
	return r
}

func (r *clientReporter) SentMessage() {
	clientStartedCounter.WithLabelValues(r.methodName).Inc()
}

func (r *clientReporter) Handled() {
	clientHandledCounter.WithLabelValues(r.methodName).Inc()
}

// ClientPrometheusPlugin for Prometheus.
type ClientPrometheusPlugin struct {
	spanMap     map[context.Context]*clientReporter
	spanMapLock sync.RWMutex
}

func NewClientPrometheusPlugin() *ClientPrometheusPlugin {
	return &ClientPrometheusPlugin{spanMap: make(map[context.Context]*clientReporter)}
}

func (p *ClientPrometheusPlugin) DoPreCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	monitor := newClientReporter(serviceMethod)
	monitor.SentMessage()

	p.spanMapLock.Lock()
	p.spanMap[ctx] = monitor
	p.spanMapLock.Unlock()

	return nil
}

func (p *ClientPrometheusPlugin) DoPostCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	var monitor *clientReporter
	p.spanMapLock.RLock()
	monitor = p.spanMap[ctx]
	p.spanMapLock.RUnlock()

	if monitor == nil {
		return nil
	}

	monitor.Handled()

	p.spanMapLock.Lock()
	delete(p.spanMap, ctx)
	p.spanMapLock.Unlock()

	return nil
}
