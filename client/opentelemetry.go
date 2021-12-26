package client

import (
	"context"

	"github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
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

func (p *OpenTelemetryPlugin) PreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	ctx, span := p.tracer.Start(ctx, "rpcx.client."+servicePath+"."+serviceMethod)
	share.Inject(ctx, p.propagators)

	span.AddEvent("PreCall")

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)
	span := trace.SpanFromContext(ctx)
	defer span.End()

	span.AddEvent("PostCall")

	return nil
}
