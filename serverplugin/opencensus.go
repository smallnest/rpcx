package serverplugin

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/server"
	"github.com/smallnest/rpcx/v5/share"
	"go.opencensus.io/trace"
)

type OpenCensusPlugin struct{}

func (p OpenCensusPlugin) Register(name string, rcvr interface{}, metadata string) error {
	_, span := trace.StartSpan(context.Background(), "rpcx.Register")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("register_service", name))

	return nil
}

func (p OpenCensusPlugin) RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	_, span := trace.StartSpan(context.Background(), "rpcx.RegisterFunction")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("register_function", serviceName+"."+fname))
	return nil
}

func (p OpenCensusPlugin) PostConnAccept(conn net.Conn) (net.Conn, bool) {
	_, span := trace.StartSpan(context.Background(), "rpcx.AcceptConn")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("remote_addr", conn.RemoteAddr().String()))
	return conn, true
}

func (p OpenCensusPlugin) PreHandleRequest(ctx context.Context, r *protocol.Message) error {
	parentContext, err := share.GetOpencensusSpanContextFromContext(ctx)
	if err != nil || parentContext == nil {
		return err
	}

	_, span1 := trace.StartSpanWithRemoteParent(ctx, "rpcx.service."+r.ServicePath+"."+r.ServiceMethod, *parentContext)
	clientConn := ctx.Value(server.RemoteConnContextKey).(net.Conn)
	span1.AddAttributes(trace.StringAttribute("remote_addr", clientConn.RemoteAddr().String()))

	if rpcxContext, ok := ctx.(*share.Context); ok {
		rpcxContext.SetValue(share.OpencensusSpanServerKey, span1)
	}
	return nil
}

func (p OpenCensusPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	if rpcxContext, ok := ctx.(*share.Context); ok {
		span1 := rpcxContext.Value(share.OpencensusSpanServerKey)
		if span1 != nil {
			span1.(*trace.Span).End()
		}
	}
	return nil
}
