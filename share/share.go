package share

import (
	"github.com/smallnest/rpcx/v5/codec"
	"github.com/smallnest/rpcx/v5/protocol"
)

const (
	// DefaultRPCPath is used by ServeHTTP.
	DefaultRPCPath = "/_rpcx_"

	// AuthKey is used in metadata.
	AuthKey = "__AUTH"

	// OpentracingSpanServerKey key in service context
	OpentracingSpanServerKey = "opentracing_span_server_key"
	// OpentracingSpanClientKey key in client context
	OpentracingSpanClientKey = "opentracing_span_client_key"

	// OpencensusSpanServerKey key in service context
	OpencensusSpanServerKey = "opencensus_span_server_key"
	// OpencensusSpanClientKey key in client context
	OpencensusSpanClientKey = "opencensus_span_client_key"
	// OpencensusSpanRequestKey span key in request meta
	OpencensusSpanRequestKey = "opencensus_span_request_key"

	// SendFileServiceName file transfer service.
	SendFileServiceName = "_filetransfer"
)

var (
	// Codecs are codecs supported by rpcx. You can add customized codecs in Codecs.
	Codecs = map[protocol.SerializeType]codec.Codec{
		protocol.SerializeNone: &codec.ByteCodec{},
		protocol.JSON:          &codec.JSONCodec{},
		protocol.ProtoBuffer:   &codec.PBCodec{},
		protocol.MsgPack:       &codec.MsgpackCodec{},
		protocol.Thrift:        &codec.ThriftCodec{},
	}
)

// RegisterCodec register customized codec.
func RegisterCodec(t protocol.SerializeType, c codec.Codec) {
	Codecs[t] = c
}

// ContextKey defines key type in context.
type ContextKey string

// ReqMetaDataKey is used to set metatdata in context of requests.
var ReqMetaDataKey = ContextKey("__req_metadata")

// ResMetaDataKey is used to set metatdata in context of responses.
var ResMetaDataKey = ContextKey("__res_metadata")

// FileTransferArgs args from clients.
type FileTransferArgs struct {
	FileName string            `json:"file_name,omitempty"`
	FileSize int64             `json:"file_size,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// FileTransferReply response to token and addr to clients.
type FileTransferReply struct {
	Token []byte `json:"token,omitempty"`
	Addr  string `json:"addr,omitempty"`
}

// DownloadFileArgs args from clients.
type DownloadFileArgs struct {
	FileName string `json:"file_name,omitempty"`
}
