package serverplugin

import (
	"context"
	"testing"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDStr = "c20ad4d76fe97759aa27a0c99bff6710"
	spanIDStr  = "6fe97759aa27a0c9"
)

var (
	traceID = mustTraceIDFromHex(traceIDStr)
	spanID  = mustSpanIDFromHex(spanIDStr)
)

func mustTraceIDFromHex(s string) (t trace.TraceID) {
	var err error
	t, err = trace.TraceIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return
}

func mustSpanIDFromHex(s string) (t trace.SpanID) {
	var err error
	t, err = trace.SpanIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return
}

func TestPreHandleRequest(t *testing.T) {
	stateStr := "key1=value1,key2=value2"
	state, err := trace.ParseTraceState(stateStr)
	if err != nil {
		t.Fatal(err)
		return
	}

	tables := []struct {
		sc trace.SpanContext
	}{
		{
			sc: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: traceID,
				SpanID:  spanID,
				Remote:  true,
			}),
		},
		{
			sc: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
				Remote:     true,
			}),
		},
		{
			trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceState: state,
				Remote:     true,
			}),
		},
		{
			trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
				TraceState: state,
				Remote:     true,
			}),
		},
	}

	for _, item := range tables {
		ctx := trace.ContextWithRemoteSpanContext(context.Background(), item.sc)
		ctx1 := share.WithValue(ctx, "", "")
		msg := protocol.NewMessage()
		tra := otel.GetTracerProvider().Tracer("rpcx")
		propagators := otel.GetTextMapPropagator()
		o := NewOpenTelemetryPlugin(tra, propagators)
		if err := o.PreHandleRequest(ctx1, msg); err != nil {
			t.Fatal(err)
			return
		}

		spanCtx := share.Extract(ctx1, propagators)

		assert.Equal(t, item.sc, spanCtx)
	}

}
