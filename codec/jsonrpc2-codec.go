package codec

import (
	"io"
	"net/rpc"

	"github.com/smallnest/rpc-codec/jsonrpc2"
)

// NewJSONRPC2ServerCodec creates a RPC-JSON 2.0 ServerCodec
func NewJSONRPC2ServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return jsonrpc2.NewServerCodec(conn, nil)
}

// NewJSONRPC2ClientCodec creates a RPC-JSON 2.0 ClientCodec
func NewJSONRPC2ClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return jsonrpc2.NewClientCodec(conn)
}
