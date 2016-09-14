package codec

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"
)

// NewJSONRPCServerCodec creates a RPC-JSON 2.0 ServerCodec
func NewJSONRPCServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return jsonrpc.NewServerCodec(conn)
}

// NewJSONRPCClientCodec creates a RPC-JSON 2.0 ClientCodec
func NewJSONRPCClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return jsonrpc.NewClientCodec(conn)
}
