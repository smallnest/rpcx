package codec

import (
	"io"
	"net/rpc"

	"github.com/mars9/codec"
)

// NewProtobufServerCodec creates a protobuf ServerCodec by https://github.com/mars9/codec
func NewProtobufServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return codec.NewServerCodec(conn)
}

// NewProtobufClientCodec creates a protobuf ClientCodec by https://github.com/mars9/codec
func NewProtobufClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return codec.NewClientCodec(conn)
}
