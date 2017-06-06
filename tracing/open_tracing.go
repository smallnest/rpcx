// refer to https://github.com/grpc-ecosystem/grpc-opentracing

package tracing

import (
	"context"

	"sync"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/smallnest/rpcx/core"
)

var (
	rpcxComponentTag = opentracing.Tag{string(ext.Component), "rpcx"}
)

// OpenTracingPlugin for opentracing and zipkin.
type OpenTracingPlugin struct {
	tracer      opentracing.Tracer
	spanMap     map[context.Context]opentracing.Span
	spanMapLock sync.RWMutex
}

func NewOpenTracingPlugin(tracer opentracing.Tracer) *OpenTracingPlugin {
	return &OpenTracingPlugin{tracer: tracer, spanMap: make(map[context.Context]opentracing.Span)}
}

func (p *OpenTracingPlugin) DoPreCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	var err error
	var parentCtx opentracing.SpanContext
	if parent := opentracing.SpanFromContext(ctx); parent != nil {
		parentCtx = parent.Context()
	}
	clientSpan := p.tracer.StartSpan(
		serviceMethod,
		opentracing.ChildOf(parentCtx),
		ext.SpanKindRPCClient,
		rpcxComponentTag,
	)
	p.spanMapLock.Lock()
	p.spanMap[ctx] = clientSpan
	p.spanMapLock.Unlock()

	header, ok := core.FromContext(ctx)
	if !ok {
		header = core.NewHeader()
	} else {
		//copy
	}
	headerWriter := headerReaderWriter{header}
	err = p.tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, headerWriter)

	// We have no better place to record an error than the Span itself :-/
	if err != nil {

		clientSpan.LogFields(log.String("event", "Tracer.Inject() failed"), log.Error(err))
	}

	return nil
}

func (p *OpenTracingPlugin) DoPostCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	var clientSpan opentracing.Span
	p.spanMapLock.RLock()
	clientSpan = p.spanMap[ctx]
	p.spanMapLock.RUnlock()

	if clientSpan == nil {
		return nil
	}

	clientSpan.Finish()

	p.spanMapLock.Lock()
	delete(p.spanMap, ctx)
	p.spanMapLock.Unlock()

	return nil
}

func (p *OpenTracingPlugin) DoPostReadRequestHeader(ctx context.Context, r *core.Request) error {
	header, ok := core.FromContext(ctx)
	if !ok {
		header = core.NewHeader()
	} else {
		//copy
	}
	headerWriter := headerReaderWriter{header}
	spanContext, err := p.tracer.Extract(opentracing.HTTPHeaders, headerWriter)
	if err != nil && err != opentracing.ErrSpanContextNotFound {
		return nil
	}
	serverSpan := p.tracer.StartSpan(
		r.ServiceMethod,
		ext.RPCServerOption(spanContext),
		rpcxComponentTag,
	)

	p.spanMapLock.Lock()
	p.spanMap[ctx] = serverSpan
	p.spanMapLock.Unlock()

	return nil
}

func (p *OpenTracingPlugin) DoPostWriteResponse(ctx context.Context, resp *core.Response, body interface{}) error {
	var serverSpan opentracing.Span
	p.spanMapLock.RLock()
	serverSpan = p.spanMap[ctx]
	p.spanMapLock.RUnlock()

	if serverSpan == nil {
		return nil
	}

	serverSpan.Finish()

	p.spanMapLock.Lock()
	delete(p.spanMap, ctx)
	p.spanMapLock.Unlock()
	return nil
}

type headerReaderWriter struct {
	core.Header
}

func (w headerReaderWriter) Set(key, val string) {
	w.Header.Set(key, val)
}

func (w headerReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range w.Header {
		for _, v := range vals {
			err := handler(k, v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
