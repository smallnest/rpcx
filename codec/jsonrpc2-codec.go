package codec

import (
	"io"

	"github.com/rpcx-ecosystem/rpc-codec2/jsonrpc2"
	"github.com/smallnest/rpcx/core"
)

// NewJSONRPC2ServerCodec creates a RPC-JSON 2.0 ServerCodec
func NewJSONRPC2ServerCodec(conn io.ReadWriteCloser) core.ServerCodec {
	return jsonrpc2.NewServerCodec(conn, nil)
}

// NewJSONRPC2ClientCodec creates a RPC-JSON 2.0 ClientCodec
func NewJSONRPC2ClientCodec(conn io.ReadWriteCloser) core.ClientCodec {
	return jsonrpc2.NewClientCodec(conn)
}
