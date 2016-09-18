package codec

import (
	colfer "github.com/pascaldekloe/colfer/rpc"
	"io"
	"net/rpc"
)

// NewColferClientCodec returns a new Colfer implementation for the core library's RPC.
func NewColferClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return colfer.NewClientCodec(conn)
}

// NewColferServerCodec returns a new Colfer implementation for the core library's RPC.
func NewColferServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return colfer.NewServerCodec(conn)
}
