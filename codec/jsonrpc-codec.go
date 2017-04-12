package codec

import (
	"io"

	"github.com/rpcx-ecosystem/jsonrpc"
	"github.com/smallnest/rpcx/core"
)

// NewJSONRPCServerCodec creates a RPC-JSON 2.0 ServerCodec
func NewJSONRPCServerCodec(conn io.ReadWriteCloser) core.ServerCodec {
	return jsonrpc.NewServerCodec(conn)
}

// NewJSONRPCClientCodec creates a RPC-JSON 2.0 ClientCodec
func NewJSONRPCClientCodec(conn io.ReadWriteCloser) core.ClientCodec {
	return jsonrpc.NewClientCodec(conn)
}
