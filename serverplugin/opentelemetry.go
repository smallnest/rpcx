package serverplugin

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type OpenTelemetryPlugin struct {
	tracer      trace.Tracer
	propagators propagation.TextMapPropagator
}

func NewOpenTelemetryPlugin(tracer trace.Tracer, propagators propagation.TextMapPropagator) *OpenTelemetryPlugin {
	if propagators == nil {
		propagators = otel.GetTextMapPropagator()
	}

	return &OpenTelemetryPlugin{
		tracer:      tracer,
		propagators: propagators,
	}
}

func (p OpenTelemetryPlugin) Register(name string, rcvr interface{}, metadata string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.Register")
	defer span.End()

	span.SetAttributes(attribute.String("register_service", name))

	return nil
}

func (p OpenTelemetryPlugin) RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.RegisterFunction")
	defer span.End()

	span.SetAttributes(attribute.String("register_function", serviceName+"."+fname))

	return nil
}

func (p OpenTelemetryPlugin) PostConnAccept(conn net.Conn) (net.Conn, bool) {
	_, span := p.tracer.Start(context.Background(), "rpcx.AcceptConn")
	defer span.End()

	span.SetAttributes(attribute.String("remote_addr", conn.RemoteAddr().String()))

	return conn, true
}

func (p OpenTelemetryPlugin) PreHandleRequest(ctx context.Context, r *protocol.Message) error {
	ctx, span := p.tracer.Start(ctx, "rpcx.service."+r.ServicePath+"."+r.ServiceMethod)
	share.Inject(ctx, p.propagators)

	span.AddEvent("PreHandleRequest")

	return nil
}

func (p OpenTelemetryPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)
	span := trace.SpanFromContext(ctx)
	defer span.End()

	span.AddEvent("PostWriteResponse")

	return nil
}
