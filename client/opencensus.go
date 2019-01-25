package client

import (
	"context"

	"github.com/smallnest/rpcx/share"
	"go.opencensus.io/trace"
)

type OpenCensusPlugin struct{}

func (p *OpenCensusPlugin) DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	var span1 *trace.Span

	// if it is called in rpc service in case that a service calls antoher service,
	// we uses the span in the service context as the parent span.
	parentSpan := ctx.Value(share.OpencensusSpanServerKey)
	if parentSpan != nil {
		_, span1 = trace.StartSpanWithRemoteParent(ctx, "rpcx.client."+servicePath+"."+serviceMethod,
			parentSpan.(*trace.Span).SpanContext())
	} else {
		parentContext, err := share.GetOpencensusSpanContextFromContext(ctx)
		if err == nil && parentContext != nil { //try to parse span from request
			_, span1 = trace.StartSpanWithRemoteParent(ctx, "rpcx.client."+servicePath+"."+serviceMethod,
				*parentContext)
		} else { // parse span from context or create root context
			_, span1 = trace.StartSpan(ctx, "rpcx.client."+servicePath+"."+serviceMethod)
		}
	}

	if rpcxContext, ok := ctx.(*share.Context); ok {
		rpcxContext.SetValue(share.OpencensusSpanClientKey, span1)
	}
	return nil
}
func (p *OpenCensusPlugin) DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	if rpcxContext, ok := ctx.(*share.Context); ok {
		span1 := rpcxContext.Value(share.OpencensusSpanClientKey)
		if span1 != nil {
			span1.(*trace.Span).End()
		}
	}
	return nil
}
