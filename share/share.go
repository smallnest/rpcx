package share

import (
	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/protocol"
)

const (
	// DefaultRPCPath is used by ServeHTTP.
	DefaultRPCPath = "/_rpcx_"

	// AuthKey is used in metadata.
	AuthKey = "__AUTH"

	// ServerAddress is used to get address of the server by client
	ServerAddress = "__ServerAddress"

	// ServerTimeout timeout value passed from client to control timeout of server
	ServerTimeout = "__ServerTimeout"

	// SendFileServiceName is name of the file transfer service.
	SendFileServiceName = "_filetransfer"

	// StreamServiceName is name of the stream service.
	StreamServiceName = "_streamservice"
)

// Trace is a flag to write a trace log or not.
// You should not enable this flag for product environment and enable it only for test.
// It writes trace log with logger Debug level.
var Trace bool

// Codecs are codecs supported by rpcx. You can add customized codecs in Codecs.
var Codecs = map[protocol.SerializeType]codec.Codec{
	protocol.SerializeNone: &codec.ByteCodec{},
	protocol.JSON:          &codec.JSONCodec{},
	protocol.ProtoBuffer:   &codec.PBCodec{},
	protocol.MsgPack:       &codec.MsgpackCodec{},
	protocol.Thrift:        &codec.ThriftCodec{},
}

// RegisterCodec register customized codec.
func RegisterCodec(t protocol.SerializeType, c codec.Codec) {
	Codecs[t] = c
}

// ContextKey defines key type in context.
type ContextKey string

// ReqMetaDataKey is used to set metadata in context of requests.
var ReqMetaDataKey = ContextKey("__req_metadata")

// ResMetaDataKey is used to set metadata in context of responses.
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
	FileName string            `json:"file_name,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// StreamServiceArgs is the request type for stream service.
type StreamServiceArgs struct {
	Meta map[string]string `json:"meta,omitempty"`
}

// StreamServiceReply is the reply type for stream service.
type StreamServiceReply struct {
	Token []byte `json:"token,omitempty"`
	Addr  string `json:"addr,omitempty"`
}
