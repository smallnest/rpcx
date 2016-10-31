package plugin

import (
	"net"

	"github.com/smallnest/rpcx"
)

// CompressionPlugin can compress responses and decompress requests
type CompressionPlugin struct {
	CompressType rpcx.CompressType
}

// NewCompressionPlugin creates a new CompressionPlugin
func NewCompressionPlugin(compressType rpcx.CompressType) *CompressionPlugin {
	return &CompressionPlugin{CompressType: compressType}
}

// HandleConnAccept can create a conn that support compression.
// Used by servers.
func (p *CompressionPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	conn = rpcx.NewCompressConn(conn, p.CompressType)
	return conn, true
}

// HandleConnected can create a conn that support compression.
// Used by servers.
func (p *CompressionPlugin) HandleConnected(conn net.Conn) (net.Conn, bool) {
	conn = rpcx.NewCompressConn(conn, p.CompressType)
	return conn, true
}

// Name return name of this plugin.
func (p *CompressionPlugin) Name() string {
	return "CompressionPlugin"
}
