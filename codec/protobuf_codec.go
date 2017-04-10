package codec

import (
	"io"

	codec "github.com/rpcx-ecosystem/codec2"
	"github.com/smallnest/rpcx/core"
)

// NewProtobufServerCodec creates a protobuf ServerCodec by https://github.com/mars9/codec
func NewProtobufServerCodec(conn io.ReadWriteCloser) core.ServerCodec {
	return codec.NewServerCodec(conn)
}

// NewProtobufClientCodec creates a protobuf ClientCodec by https://github.com/mars9/codec
func NewProtobufClientCodec(conn io.ReadWriteCloser) core.ClientCodec {
	return codec.NewClientCodec(conn)
}
