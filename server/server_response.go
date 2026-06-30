package server

import (
	"context"
	"net"
	"time"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// Response sending and direct message sending for Server.
// Extracted from server.go.

// SendMessage a request to the specified client.
// The client is designated by the conn.
// conn can be gotten from context in services:
//
//	ctx.Value(RemoteConnContextKey)
//
// servicePath, serviceMethod, metadata can be set to zero values.
func (s *Server) SendMessage(conn net.Conn, servicePath, serviceMethod string, metadata map[string]string, data []byte) error {
	ctx := share.WithValue(context.Background(), StartSendRequestContextKey, time.Now().UnixNano())
	s.Plugins.DoPreWriteRequest(ctx)

	req := protocol.NewMessage()
	req.SetMessageType(protocol.Request)

	seq := s.seq.Add(1)
	req.SetSeq(seq)
	req.SetOneway(true)
	req.SetSerializeType(protocol.SerializeNone)
	req.ServicePath = servicePath
	req.ServiceMethod = serviceMethod
	req.Metadata = metadata
	req.Payload = data

	b := req.EncodeSlicePointer()
	_, err := conn.Write(*b)
	protocol.PutData(b)

	s.Plugins.DoPostWriteRequest(ctx, req, err)

	return err
}

func (s *Server) sendResponse(ctx *share.Context, conn net.Conn, err error, req, res *protocol.Message) {
	if len(res.Payload) > 1024 && req.CompressType() != protocol.None {
		res.SetCompressType(req.CompressType())
	}

	s.Plugins.DoPreWriteResponse(ctx, req, res, err)

	data := res.EncodeSlicePointer()
	if s.AsyncWrite {
		if s.pool != nil {
			s.pool.Submit(func() {
				if s.writeTimeout != 0 {
					conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
				}
				conn.Write(*data)
				protocol.PutData(data)
			})
		} else {
			go func() {
				if s.writeTimeout != 0 {
					conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
				}
				conn.Write(*data)
				protocol.PutData(data)
			}()
		}
	} else {
		if s.writeTimeout != 0 {
			conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
		}
		conn.Write(*data)
		protocol.PutData(data)
	}
	s.Plugins.DoPostWriteResponse(ctx, req, res, err)
}
