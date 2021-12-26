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
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithSpanContext(ctx, spanCtx)

	ctx1, span := p.tracer.Start(ctx0, "rpcx.client."+servicePath+"."+serviceMethod)
	share.Inject(ctx1, p.propagators)

	ctx.(*share.Context).SetValue(share.OpenTelemetryKey, span)

	span.AddEvent("PreCall")

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.AddEvent("PostCall")

	return nil
}
