package codec

import (
	"io"

	gencodec "github.com/rpcx-ecosystem/net-rpc-gencode2"
	"github.com/smallnest/rpcx/core"
)

// NewGencodeServerCodec creates a gencode ServerCodec
func NewGencodeServerCodec(conn io.ReadWriteCloser) core.ServerCodec {
	return gencodec.NewGencodeServerCodec(conn)
}

// NewGencodeClientCodec creates a gencode ClientCodec
func NewGencodeClientCodec(conn io.ReadWriteCloser) core.ClientCodec {
	return gencodec.NewGencodeClientCodec(conn)
}
