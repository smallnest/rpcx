package serverplugin

import (
	"io"
	"net"
)

// TeeConnPlugin is a plugin that copy requests from clients and send to a io.Writer.
type TeeConnPlugin struct {
	w io.Writer
}

func NewTeeConnPlugin(w io.Writer) *TeeConnPlugin {
	return &TeeConnPlugin{w: w}
}

// Update can start a stream copy by setting a non-nil w.
// If you set a nil w, it doesn't copy stream.
func (plugin *TeeConnPlugin) Update(w io.Writer) {
	plugin.w = w
}

// HandleConnAccept check ip.
func (plugin *TeeConnPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	tc := &teeConn{conn, plugin.w}
	return tc, true
}

type teeConn struct {
	net.Conn
	w io.Writer
}

func (t *teeConn) Read(p []byte) (n int, err error) {
	n, err = t.Conn.Read(p)
	if n > 0 && t.w != nil {
		if _, err := t.w.Write(p[:n]); err != nil {
			// return n, err //discard error
		}
	}
	return
}
