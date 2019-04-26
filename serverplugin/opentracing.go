package serverplugin

import (
	"context"
	"net"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
	"github.com/smallnest/rpcx/share"
)

type OpenTracingPlugin struct{}

func (p OpenTracingPlugin) Register(name string, rcvr interface{}, metadata string) error {
	span1 := opentracing.StartSpan(
		"rpcx.Register")
	defer span1.Finish()

	span1.LogFields(log.String("register_service", name))

	return nil
}

func (p OpenTracingPlugin) RegisterFunction(name string, fn interface{}, metadata string) error {
	span1 := opentracing.StartSpan(
		"rpcx.RegisterFunction")
	defer span1.Finish()

	span1.LogFields(log.String("register_function", name))
	return nil
}

func (p OpenTracingPlugin) PostConnAccept(conn net.Conn) (net.Conn, bool) {
	span1 := opentracing.StartSpan(
		"rpcx.AcceptConn")
	defer span1.Finish()

	span1.LogFields(log.String("remote_addr", conn.RemoteAddr().String()))
	return conn, true
}

func (p OpenTracingPlugin) PreHandleRequest(ctx context.Context, r *protocol.Message) error {
	wireContext, err := share.GetSpanContextFromContext(ctx)
	if err != nil || wireContext == nil {
		return err
	}
	span1 := opentracing.StartSpan(
		"rpcx.service."+r.ServicePath+"."+r.ServiceMethod,
		ext.RPCServerOption(wireContext))

	clientConn := ctx.Value(server.RemoteConnContextKey).(net.Conn)
	span1.LogFields(log.String("remote_addr", clientConn.RemoteAddr().Network()))

	if rpcxContext, ok := ctx.(*share.Context); ok {
		rpcxContext.SetValue(share.OpentracingSpanServerKey, span1)
	}
	return nil
}

func (p OpenTracingPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	if rpcxContext, ok := ctx.(*share.Context); ok {
		span1 := rpcxContext.Value(share.OpentracingSpanServerKey)
		if span1 != nil {
			span1.(opentracing.Span).Finish()
		}
	}
	return nil
}
