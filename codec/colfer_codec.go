package codec

import (
	"io"

	colfer "github.com/rpcx-ecosystem/colfer/rpc"
	"github.com/smallnest/rpcx/core"
)

// NewColferClientCodec returns a new Colfer implementation for the core library's core.
func NewColferClientCodec(conn io.ReadWriteCloser) core.ClientCodec {
	return colfer.NewClientCodec(conn)
}

// NewColferServerCodec returns a new Colfer implementation for the core library's core.
func NewColferServerCodec(conn io.ReadWriteCloser) core.ServerCodec {
	return colfer.NewServerCodec(conn)
}
