package rpcx

import (
	"compress/flate"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/golang/snappy"
)

// CompressType is compression type. Currently only support zip and snappy
type CompressType byte

const (
	// CompressNone represents no compression
	CompressNone CompressType = iota
	// CompressFlate represents zip
	CompressFlate
	// CompressSnappy represents snappy
	CompressSnappy
)

type writeFlusher struct {
	w *flate.Writer
}

func (wf *writeFlusher) Write(p []byte) (int, error) {
	n, err := wf.w.Write(p)
	if err != nil {
		return n, err
	}
	if err := wf.w.Flush(); err != nil {
		return 0, err
	}
	return n, nil
}

// CompressConn wraps a net.Conn and supports compression
type CompressConn struct {
	conn         net.Conn
	r            io.Reader
	w            io.Writer
	compressType CompressType
}

// NewCompressConn creates a wrapped net.Conn with CompressType
func NewCompressConn(conn net.Conn, compressType CompressType) net.Conn {
	cc := &CompressConn{conn: conn}
	r := io.Reader(cc.conn)

	switch compressType {
	case CompressNone:
	case CompressFlate:
		r = flate.NewReader(r)
	case CompressSnappy:
		r = snappy.NewReader(r)
	}
	cc.r = r

	w := io.Writer(cc.conn)
	switch compressType {
	case CompressNone:
	case CompressFlate:
		zw, err := flate.NewWriter(w, flate.DefaultCompression)
		if err != nil {
			panic(fmt.Sprintf("BUG: flate.NewWriter(%d) returned non-nil err: %s", flate.DefaultCompression, err))
		}
		w = &writeFlusher{w: zw}
	case CompressSnappy:
		w = snappy.NewWriter(w)
	}
	cc.w = w

	return cc
}

func (c *CompressConn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func (c *CompressConn) Write(b []byte) (n int, err error) {
	return c.w.Write(b)
}

// Close closes the connection.
func (c *CompressConn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local network address.
func (c *CompressConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *CompressConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
func (c *CompressConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls.
func (c *CompressConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls.
func (c *CompressConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
