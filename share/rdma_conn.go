//go:build rdma
// +build rdma

package share

import (
	"net"
	"time"

	"github.com/smallnest/gordma/rdmanet"
)

// RDMAAddr adapts an rdmanet string address to net.Addr. Its Network is always
// "rdma".
type RDMAAddr string

// Network returns "rdma".
func (a RDMAAddr) Network() string { return "rdma" }

// String returns the underlying rdmanet address string.
func (a RDMAAddr) String() string { return string(a) }

// RDMAConn adapts *rdmanet.Conn to net.Conn. rdmanet.Conn already implements
// Read/Write/Close with built-in message framing and credit-based flow
// control, so those methods pass through unchanged. This adapter only bridges
// the remaining net.Conn gaps: LocalAddr/RemoteAddr return a net.Addr instead
// of a string, and the SetDeadline family is supplied as a documented no-op
// (rdmanet.Conn has no native deadline support).
type RDMAConn struct {
	*rdmanet.Conn
}

var _ net.Conn = (*RDMAConn)(nil)

// NewRDMAConn wraps an *rdmanet.Conn as a net.Conn.
func NewRDMAConn(c *rdmanet.Conn) *RDMAConn { return &RDMAConn{Conn: c} }

// LocalAddr returns the local endpoint as a net.Addr.
func (c *RDMAConn) LocalAddr() net.Addr { return RDMAAddr(c.Conn.LocalAddr()) }

// RemoteAddr returns the remote endpoint as a net.Addr.
func (c *RDMAConn) RemoteAddr() net.Addr { return RDMAAddr(c.Conn.RemoteAddr()) }

// SetDeadline is a no-op: rdmanet.Conn has no native deadline support. It
// returns nil so callers that set deadlines (such as rpcx's call paths) are
// not disrupted, but the deadline is not enforced.
func (c *RDMAConn) SetDeadline(t time.Time) error { return nil }

// SetReadDeadline is a no-op; see SetDeadline.
func (c *RDMAConn) SetReadDeadline(t time.Time) error { return nil }

// SetWriteDeadline is a no-op; see SetDeadline.
func (c *RDMAConn) SetWriteDeadline(t time.Time) error { return nil }

// RDMAListener adapts *rdmanet.Listener to net.Listener. Accept wraps each
// accepted *rdmanet.Conn in an RDMAConn, and Addr bridges the string address
// to a net.Addr.
type RDMAListener struct {
	*rdmanet.Listener
}

var _ net.Listener = (*RDMAListener)(nil)

// NewRDMAListener wraps an *rdmanet.Listener as a net.Listener.
func NewRDMAListener(l *rdmanet.Listener) *RDMAListener { return &RDMAListener{Listener: l} }

// Accept waits for and returns the next connection as a net.Conn.
func (l *RDMAListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewRDMAConn(c), nil
}

// Addr returns the listener's network address as a net.Addr.
func (l *RDMAListener) Addr() net.Addr { return RDMAAddr(l.Listener.Addr()) }
