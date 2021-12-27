package serverplugin

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
	"github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	_ server.RegisterPlugin          = (*OpenTelemetryPlugin)(nil)
	_ server.PostConnAcceptPlugin    = (*OpenTelemetryPlugin)(nil)
	_ server.PreHandleRequestPlugin  = (*OpenTelemetryPlugin)(nil)
	_ server.PostWriteResponsePlugin = (*OpenTelemetryPlugin)(nil)
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

func (p OpenTelemetryPlugin) Unregister(name string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.Unregister")
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

func (p OpenTelemetryPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	_, span := p.tracer.Start(context.Background(), "rpcx.AcceptConn")
	defer span.End()

	span.SetAttributes(attribute.String("remote_addr", conn.RemoteAddr().String()))

	return conn, true
}

func (p OpenTelemetryPlugin) PreHandleRequest(ctx context.Context, r *protocol.Message) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithSpanContext(ctx, spanCtx)

	ctx1, span := p.tracer.Start(ctx0, "rpcx.service."+r.ServicePath+"."+r.ServiceMethod)
	share.Inject(ctx1, p.propagators)

	ctx.(*share.Context).SetValue(share.OpenTelemetryKey, span)

	span.AddEvent("PreHandleRequest")

	return nil
}

func (p OpenTelemetryPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.AddEvent("PostWriteResponse")

	return nil
}
