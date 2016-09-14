package codec

import (
	"io"
	"net/rpc"

	gencodec "github.com/smallnest/net-rpc-gencode"
)

// NewGencodeServerCodec creates a gencode ServerCodec
func NewGencodeServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return gencodec.NewGencodeServerCodec(conn)
}

// NewGencodeClientCodec creates a gencode ClientCodec
func NewGencodeClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return gencodec.NewGencodeClientCodec(conn)
}
