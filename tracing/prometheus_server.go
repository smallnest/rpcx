package tracing

import (
	"context"
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/smallnest/rpcx/core"
)

var (
	serverStartedCounter = prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "rpcx",
			Subsystem: "server",
			Name:      "started_total",
			Help:      "Total number of RPCs started on the server.",
		}, []string{"rpcx_method"})

	serverHandledCounter = prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "rpcx",
			Subsystem: "server",
			Name:      "handled_total",
			Help:      "Total number of RPCs completed on the server, regardless of success or failure.",
		}, []string{"rpcx_method"})
)

func init() {
	prom.MustRegister(serverStartedCounter)
	prom.MustRegister(serverHandledCounter)
}

type serverReporter struct {
	methodName string
	startTime  time.Time
}

func newServerReporter(method string) *serverReporter {
	r := &serverReporter{}
	r.methodName = method
	serverStartedCounter.WithLabelValues(r.methodName).Inc()
	return r
}

func (r *serverReporter) ReceivedMessage() {
	serverStartedCounter.WithLabelValues(r.methodName).Inc()
}

func (r *serverReporter) Handled() {
	serverHandledCounter.WithLabelValues(r.methodName).Inc()
}

// ServerPrometheusPlugin for Prometheus.
type ServerPrometheusPlugin struct {
	spanMap     map[context.Context]*serverReporter
	spanMapLock sync.RWMutex
}

func NewServerPrometheusPlugin() *ServerPrometheusPlugin {
	return &ServerPrometheusPlugin{spanMap: make(map[context.Context]*serverReporter)}
}

func (p *ServerPrometheusPlugin) DoPostReadRequestHeader(ctx context.Context, r *core.Request) error {
	monitor := newServerReporter(r.ServiceMethod)
	monitor.ReceivedMessage()

	p.spanMapLock.Lock()
	p.spanMap[ctx] = monitor
	p.spanMapLock.Unlock()

	return nil
}

func (p *ServerPrometheusPlugin) DoPostWriteResponse(ctx context.Context, resp *core.Response, body interface{}) error {
	var monitor *serverReporter
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
