package share

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	opentracing "github.com/opentracing/opentracing-go"
	"go.opencensus.io/trace"
)

// var _ context.Context = &Context{}

// Context is a rpcx customized Context that can contains multiple values.
type Context struct {
	tags map[interface{}]interface{}
	context.Context
}

func NewContext(ctx context.Context) *Context {
	return &Context{
		Context: ctx,
		tags:    make(map[interface{}]interface{}),
	}
}

func (c *Context) Value(key interface{}) interface{} {
	if c.tags == nil {
		c.tags = make(map[interface{}]interface{})
	}

	if v, ok := c.tags[key]; ok {
		return v
	}
	return c.Context.Value(key)
}

func (c *Context) SetValue(key, val interface{}) {
	if c.tags == nil {
		c.tags = make(map[interface{}]interface{})
	}
	c.tags[key] = val
}

func (c *Context) String() string {
	return fmt.Sprintf("%v.WithValue(%v)", c.Context, c.tags)
}

func WithValue(parent context.Context, key, val interface{}) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	tags := make(map[interface{}]interface{})
	tags[key] = val
	return &Context{Context: parent, tags: tags}
}

func WithLocalValue(ctx *Context, key, val interface{}) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	if ctx.tags == nil {
		ctx.tags = make(map[interface{}]interface{})
	}

	ctx.tags[key] = val
	return ctx
}

// GetSpanContextFromContext get opentracing.SpanContext from context.Context.
func GetSpanContextFromContext(ctx context.Context) (opentracing.SpanContext, error) {
	reqMeta, ok := ctx.Value(ReqMetaDataKey).(map[string]string)
	if !ok {
		return nil, nil
	}
	return opentracing.GlobalTracer().Extract(
		opentracing.TextMap,
		opentracing.TextMapCarrier(reqMeta))
}

// GetOpencensusSpanContextFromContext get opencensus.trace.SpanContext from context.Context.
func GetOpencensusSpanContextFromContext(ctx context.Context) (*trace.SpanContext, error) {
	reqMeta, ok := ctx.Value(ReqMetaDataKey).(map[string]string)
	if !ok {
		return nil, nil
	}
	spanKey := reqMeta[OpencensusSpanRequestKey]
	if spanKey == "" {
		return nil, errors.New("key not found")
	}

	data := []byte(spanKey)
	_ = data[23]

	t := &trace.SpanContext{}
	copy(t.TraceID[:], data[:16])
	copy(t.SpanID[:], data[16:24])

	return t, nil
}
