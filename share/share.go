package share

import (
	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/protocol"
)

const (
	// DefaultRPCPath is used by ServeHTTP
	DefaultRPCPath = "/_rpcx_"
)

var (
	// Codecs are codecs supported by rpcx.
	Codecs = map[protocol.SerializeType]codec.Codec{
		protocol.SerializeNone: &codec.ByteCodec{},
		protocol.JSON:          &codec.JSONCodec{},
		protocol.ProtoBuffer:   &codec.PBCodec{},
		protocol.MsgPack:       &codec.MsgpackCodec{},
	}
)
