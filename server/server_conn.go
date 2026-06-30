package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/soheilhy/cmux"
)

// Connection accept loop and per-connection request handling for Server.
// Extracted from server.go.

// serveListener accepts incoming connections on the Listener ln,
// creating a new service goroutine for each.
// The service goroutines read requests and then call services to reply to them.
func (s *Server) serveListener(ln net.Listener) error {
	var tempDelay time.Duration

	s.mu.Lock()
	s.ln = ln
	close(s.Started)
	s.mu.Unlock()

	for {
		conn, e := ln.Accept()
		if e != nil {
			if s.isShutdown() {
				<-s.doneChan
				return ErrServerClosed
			}

			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Errorf("rpcx: Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			if errors.Is(e, cmux.ErrListenerClosed) {
				return ErrServerClosed
			}
			return e
		}
		tempDelay = 0

		if tc, ok := conn.(*net.TCPConn); ok {
			period := s.options["TCPKeepAlivePeriod"]
			if period != nil {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(period.(time.Duration))
				tc.SetLinger(10)
			}
		}

		conn, ok := s.Plugins.DoPostConnAccept(conn)
		if !ok {
			conn.Close()
			continue
		}

		s.mu.Lock()
		s.activeConn[conn] = struct{}{}
		s.mu.Unlock()

		if share.Trace {
			log.Debugf("server accepted an conn: %v", conn.RemoteAddr().String())
		}

		go s.serveConn(conn)
	}
}

func (s *Server) serveConn(conn net.Conn) {
	if s.isShutdown() {
		s.closeConn(conn)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			ss := runtime.Stack(buf, false)
			if ss > size {
				ss = size
			}
			buf = buf[:ss]
			log.Errorf("serving %s panic error: %s, stack:\n %s", conn.RemoteAddr(), err, buf)
		}

		if share.Trace {
			log.Debugf("server closed conn: %v", conn.RemoteAddr().String())
		}

		// make sure all inflight requests are handled and all drained
		if s.isShutdown() {
			<-s.doneChan
		}

		s.closeConn(conn)
	}()

	if tlsConn, ok := conn.(*tls.Conn); ok {
		if d := s.readTimeout; d != 0 {
			conn.SetReadDeadline(time.Now().Add(d))
		}
		if d := s.writeTimeout; d != 0 {
			conn.SetWriteDeadline(time.Now().Add(d))
		}
		if err := tlsConn.Handshake(); err != nil {
			log.Errorf("rpcx: TLS handshake error from %s: %v", conn.RemoteAddr(), err)
			return
		}
	}

	r := bufio.NewReaderSize(conn, ReaderBuffsize)

	// read requests and handle it
	for {
		if s.isShutdown() {
			return
		}

		t0 := time.Now()
		if s.readTimeout != 0 {
			conn.SetReadDeadline(t0.Add(s.readTimeout))
		}

		// create a rpcx Context
		ctx := share.WithValue(context.Background(), RemoteConnContextKey, conn)

		// read a request from the underlying connection
		req, err := s.readRequest(ctx, r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Infof("client has closed this connection: %s", conn.RemoteAddr().String())
			} else if errors.Is(err, net.ErrClosed) {
				log.Infof("rpcx: connection %s is closed", conn.RemoteAddr().String())
			} else if errors.Is(err, ErrReqReachLimit) {
				if !req.IsOneway() { // return a error response
					res := req.Clone()
					res.SetMessageType(protocol.Response)

					s.handleError(res, err)
					s.sendResponse(ctx, conn, err, req, res)
				} else { // Oneway and only call the plugins
					s.Plugins.DoPreWriteResponse(ctx, req, nil, err)
				}
				continue
			} else { // wrong data
				log.Warnf("rpcx: failed to read request: %v", err)
			}

			if s.HandleServiceError != nil {
				s.HandleServiceError(err)
			}

			return
		}

		if share.Trace {
			log.Debugf("server received an request %+v from conn: %v", req, conn.RemoteAddr().String())
		}

		ctx = share.WithLocalValue(ctx, StartRequestContextKey, time.Now().UnixNano())
		closeConn := false
		if !req.IsHeartbeat() {
			err = s.auth(ctx, req)
			closeConn = err != nil
		}

		if err != nil {
			if !req.IsOneway() { // return a error response
				res := req.Clone()
				res.SetMessageType(protocol.Response)
				s.handleError(res, err)
				s.sendResponse(ctx, conn, err, req, res)
			} else {
				s.Plugins.DoPreWriteResponse(ctx, req, nil, err)
			}

			if s.HandleServiceError != nil {
				s.HandleServiceError(err)
			}

			// auth failed, closed the connection
			if closeConn {
				log.Infof("auth failed for conn %s: %v", conn.RemoteAddr().String(), err)
				return
			}
			continue
		}

		if s.pool != nil {
			s.pool.Submit(func() {
				s.processOneRequest(ctx, req, conn)
			})
		} else {
			go s.processOneRequest(ctx, req, conn)
		}
	}
}

func (s *Server) processOneRequest(ctx *share.Context, req *protocol.Message, conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 1024)
			buf = buf[:runtime.Stack(buf, true)]
			if s.HandleServiceError != nil {
				s.HandleServiceError(fmt.Errorf("%v", r))
			} else {
				log.Errorf("[handler internal error]: servicepath: %s, servicemethod: %s, err: %v，stacks: %s", req.ServicePath, req.ServiceMethod, r, string(buf))
			}
			sctx := NewContext(ctx, conn, req, s.AsyncWrite)
			sctx.WriteError(fmt.Errorf("%v", r))
		}
	}()

	atomic.AddInt32(&s.handlerMsgNum, 1)
	defer atomic.AddInt32(&s.handlerMsgNum, -1)

	// 心跳请求，直接处理返回
	if req.IsHeartbeat() {
		s.Plugins.DoHeartbeatRequest(ctx, req)
		req.SetMessageType(protocol.Response)
		data := req.EncodeSlicePointer()

		if s.writeTimeout != 0 {
			conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
		}
		conn.Write(*data)

		protocol.PutData(data)

		return
	}

	cancelFunc := parseServerTimeout(ctx, req)
	if cancelFunc != nil {
		defer cancelFunc()
	}

	resMetadata := make(map[string]string)
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}
	ctx = share.WithLocalValue(share.WithLocalValue(ctx, share.ReqMetaDataKey, req.Metadata),
		share.ResMetaDataKey, resMetadata)

	s.Plugins.DoPreHandleRequest(ctx, req)

	if share.Trace {
		log.Debugf("server handle request %+v from conn: %v", req, conn.RemoteAddr().String())
	}

	// use handlers first
	if handler, ok := s.router[req.ServicePath+"."+req.ServiceMethod]; ok {
		sctx := NewContext(ctx, conn, req, s.AsyncWrite)
		err := handler(sctx)
		if err != nil {
			if s.HandleServiceError != nil {
				s.HandleServiceError(err)
			} else {
				log.Errorf("[handler internal error]: servicepath: %s, servicemethod: %s, err: %v", req.ServicePath, req.ServiceMethod, err)
			}
			sctx.WriteError(err)
		}

		return
	}

	res, err := s.handleRequest(ctx, req)
	if err != nil {
		if s.HandleServiceError != nil {
			s.HandleServiceError(err)
		} else {
			log.Warnf("rpcx: failed to handle request: %v", err)
		}
	}

	if !req.IsOneway() {
		if len(resMetadata) > 0 { // copy meta in context to responses
			meta := res.Metadata
			if meta == nil {
				res.Metadata = resMetadata
			} else {
				for k, v := range resMetadata {
					if meta[k] == "" {
						meta[k] = v
					}
				}
			}
		}

		s.sendResponse(ctx, conn, err, req, res)
	}

	if share.Trace {
		log.Debugf("server write response %+v for an request %+v from conn: %v", res, req, conn.RemoteAddr().String())
	}
}

func parseServerTimeout(ctx *share.Context, req *protocol.Message) context.CancelFunc {
	if req == nil || req.Metadata == nil {
		return nil
	}

	st := req.Metadata[share.ServerTimeout]
	if st == "" {
		return nil
	}

	timeout, err := strconv.ParseInt(st, 10, 64)
	if err != nil {
		return nil
	}

	newCtx, cancel := context.WithTimeout(ctx.Context, time.Duration(timeout)*time.Millisecond)
	ctx.Context = newCtx
	return cancel
}

func (s *Server) isShutdown() bool {
	return atomic.LoadInt32(&s.inShutdown) == 1
}

func (s *Server) closeConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.activeConn, conn)
	s.mu.Unlock()

	conn.Close()

	s.Plugins.DoPostConnClose(conn)
}

func (s *Server) readRequest(ctx context.Context, r io.Reader) (req *protocol.Message, err error) {
	err = s.Plugins.DoPreReadRequest(ctx)
	if err != nil {
		return nil, err
	}
	// pool req?
	req = protocol.NewMessage()
	err = req.Decode(r)
	if err == io.EOF {
		return req, err
	}
	perr := s.Plugins.DoPostReadRequest(ctx, req, err)
	if err == nil {
		err = perr
	}
	return req, err
}

func (s *Server) auth(ctx context.Context, req *protocol.Message) error {
	if s.AuthFunc != nil {
		token := req.Metadata[share.AuthKey]
		return s.AuthFunc(ctx, req, token)
	}

	return nil
}
