package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/soheilhy/cmux"
	"golang.org/x/net/websocket"
)

// ErrServerClosed is returned by the Server's Serve, ListenAndServe after a call to Shutdown or Close.
var (
	ErrServerClosed  = errors.New("http: Server closed")
	ErrReqReachLimit = errors.New("request reached rate limit")
)

const (
	// ReaderBuffsize is used for bufio reader.
	ReaderBuffsize = 1024
	// WriterBuffsize is used for bufio writer.
	WriterBuffsize = 1024

	// // WriteChanSize is used for response.
	// WriteChanSize = 1024 * 1024
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "rpcx context value " + k.name }

var (
	// RemoteConnContextKey is a context key. It can be used in
	// services with context.WithValue to access the connection arrived on.
	// The associated value will be of type net.Conn.
	RemoteConnContextKey = &contextKey{"remote-conn"}
	// StartRequestContextKey records the start time
	StartRequestContextKey = &contextKey{"start-parse-request"}
	// StartSendRequestContextKey records the start time
	StartSendRequestContextKey = &contextKey{"start-send-request"}
	// TagContextKey is used to record extra info in handling services. Its value is a map[string]interface{}
	TagContextKey = &contextKey{"service-tag"}
	// HttpConnContextKey is used to store http connection.
	HttpConnContextKey = &contextKey{"http-conn"}
)

type Handler func(ctx *Context) error

type WorkerPool interface {
	Submit(task func())
	StopAndWaitFor(deadline time.Duration)
	Stop()
	StopAndWait()
}

// Server is rpcx server that use TCP or UDP.
type Server struct {
	ln                 net.Listener
	readTimeout        time.Duration
	writeTimeout       time.Duration
	gatewayHTTPServer  *http.Server
	jsonrpcHTTPServer  *http.Server
	DisableHTTPGateway bool // disable http invoke or not.
	DisableJSONRPC     bool // disable json rpc or not.
	AsyncWrite         bool // set true if your server only serves few clients
	pool               WorkerPool

	serviceMapMu sync.RWMutex
	serviceMap   map[string]*service

	router map[string]Handler

	mu         sync.RWMutex
	activeConn map[net.Conn]struct{}
	doneChan   chan struct{}
	seq        uint64

	inShutdown int32
	onShutdown []func(s *Server)
	onRestart  []func(s *Server)

	// TLSConfig for creating tls tcp connection.
	tlsConfig *tls.Config
	// BlockCrypt for kcp.BlockCrypt
	options map[string]interface{}

	// CORS options
	corsOptions *CORSOptions

	Plugins PluginContainer

	// AuthFunc can be used to auth.
	AuthFunc func(ctx context.Context, req *protocol.Message, token string) error

	handlerMsgNum int32

	// HandleServiceError is used to get all service errors. You can use it write logs or others.
	HandleServiceError func(error)

	// ServerErrorFunc is a customized error handlers and you can use it to return customized error strings to clients.
	// If not set, it use err.Error()
	ServerErrorFunc func(res *protocol.Message, err error) string
}

// NewServer returns a server.
func NewServer(options ...OptionFn) *Server {
	s := &Server{
		Plugins:    &pluginContainer{},
		options:    make(map[string]interface{}),
		activeConn: make(map[net.Conn]struct{}),
		doneChan:   make(chan struct{}),
		serviceMap: make(map[string]*service),
		router:     make(map[string]Handler),
		AsyncWrite: false, // 除非你想做进一步的优化测试，否则建议你设置为false
	}

	for _, op := range options {
		op(s)
	}

	if s.options["TCPKeepAlivePeriod"] == nil {
		s.options["TCPKeepAlivePeriod"] = 3 * time.Minute
	}
	return s
}

// Address returns listened address.
func (s *Server) Address() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

func (s *Server) AddHandler(servicePath, serviceMethod string, handler func(*Context) error) {
	s.router[servicePath+"."+serviceMethod] = handler
}

// ActiveClientConn returns active connections.
func (s *Server) ActiveClientConn() []net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]net.Conn, 0, len(s.activeConn))
	for clientConn := range s.activeConn {
		result = append(result, clientConn)
	}
	return result
}

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

	seq := atomic.AddUint64(&s.seq, 1)
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

func (s *Server) getDoneChan() <-chan struct{} {
	return s.doneChan
}

// Serve starts and listens RPC requests.
// It is blocked until receiving connections from clients.
func (s *Server) Serve(network, address string) (err error) {
	var ln net.Listener
	ln, err = s.makeListener(network, address)
	if err != nil {
		return err
	}

	return s.ServeListener(network, ln)
}

// ServeListener listens RPC requests.
// It is blocked until receiving connections from clients.
func (s *Server) ServeListener(network string, ln net.Listener) (err error) {
	if network == "http" {
		s.serveByHTTP(ln, "")
		return nil
	}

	if network == "ws" || network == "wss" {
		s.serveByWS(ln, "")
		return nil
	}

	// try to start gateway
	ln = s.startGateway(network, ln)

	return s.serveListener(ln)
}

// serveListener accepts incoming connections on the Listener ln,
// creating a new service goroutine for each.
// The service goroutines read requests and then call services to reply to them.
func (s *Server) serveListener(ln net.Listener) error {
	var tempDelay time.Duration

	s.mu.Lock()
	s.ln = ln
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

// serveByHTTP serves by HTTP.
// if rpcPath is an empty string, use share.DefaultRPCPath.
func (s *Server) serveByHTTP(ln net.Listener, rpcPath string) {
	s.ln = ln

	if rpcPath == "" {
		rpcPath = share.DefaultRPCPath
	}
	mux := http.NewServeMux()
	mux.Handle(rpcPath, s)
	srv := &http.Server{Handler: mux}

	srv.Serve(ln)
}

func (s *Server) serveByWS(ln net.Listener, rpcPath string) {
	s.ln = ln

	if rpcPath == "" {
		rpcPath = share.DefaultRPCPath
	}
	mux := http.NewServeMux()
	mux.Handle(rpcPath, websocket.Handler(s.ServeWS))
	srv := &http.Server{Handler: mux}

	srv.Serve(ln)
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

			log.Errorf("failed to handle the request: %v， stacks: %s", r, buf)
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
			log.Errorf("[handler internal error]: servicepath: %s, servicemethod, err: %v", req.ServicePath, req.ServiceMethod, err)
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

func (s *Server) handleRequest(ctx context.Context, req *protocol.Message) (res *protocol.Message, err error) {
	serviceName := req.ServicePath
	methodName := req.ServiceMethod

	res = req.Clone()

	res.SetMessageType(protocol.Response)
	s.serviceMapMu.RLock()
	service := s.serviceMap[serviceName]

	if share.Trace {
		log.Debugf("server get service %+v for an request %+v", service, req)
	}

	s.serviceMapMu.RUnlock()
	if service == nil {
		err = errors.New("rpcx: can't find service " + serviceName)
		return s.handleError(res, err)
	}
	mtype := service.method[methodName]
	if mtype == nil {
		if service.function[methodName] != nil { // check raw functions
			return s.handleRequestForFunction(ctx, req)
		}
		err = errors.New("rpcx: can't find method " + methodName)
		return s.handleError(res, err)
	}

	// get a argv object from object pool
	argv := reflectTypePools.Get(mtype.ArgType)

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		err = fmt.Errorf("can not find codec for %d", req.SerializeType())
		return s.handleError(res, err)
	}

	err = codec.Decode(req.Payload, argv)
	if err != nil {
		return s.handleError(res, err)
	}

	// and get a reply object from object pool
	replyv := reflectTypePools.Get(mtype.ReplyType)

	argv, err = s.Plugins.DoPreCall(ctx, serviceName, methodName, argv)
	if err != nil {
		// return reply to object pool
		reflectTypePools.Put(mtype.ReplyType, replyv)
		return s.handleError(res, err)
	}

	if mtype.ArgType.Kind() != reflect.Ptr {
		err = service.call(ctx, mtype, reflect.ValueOf(argv).Elem(), reflect.ValueOf(replyv))
	} else {
		err = service.call(ctx, mtype, reflect.ValueOf(argv), reflect.ValueOf(replyv))
	}

	if err == nil {
		replyv, err = s.Plugins.DoPostCall(ctx, serviceName, methodName, argv, replyv)
	}

	// return argc to object pool
	reflectTypePools.Put(mtype.ArgType, argv)

	if err != nil {
		if replyv != nil {
			data, err := codec.Encode(replyv)
			// return reply to object pool
			reflectTypePools.Put(mtype.ReplyType, replyv)
			if err != nil {
				return s.handleError(res, err)
			}
			res.Payload = data
		}
		return s.handleError(res, err)
	}

	if !req.IsOneway() {
		data, err := codec.Encode(replyv)
		// return reply to object pool
		reflectTypePools.Put(mtype.ReplyType, replyv)
		if err != nil {
			return s.handleError(res, err)
		}
		res.Payload = data
	} else if replyv != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
	}

	if share.Trace {
		log.Debugf("server called service %+v for an request %+v", service, req)
	}

	return res, nil
}

func (s *Server) handleRequestForFunction(ctx context.Context, req *protocol.Message) (res *protocol.Message, err error) {
	res = req.Clone()

	res.SetMessageType(protocol.Response)

	serviceName := req.ServicePath
	methodName := req.ServiceMethod
	s.serviceMapMu.RLock()
	service := s.serviceMap[serviceName]
	s.serviceMapMu.RUnlock()
	if service == nil {
		err = errors.New("rpcx: can't find service  for func raw function")
		return s.handleError(res, err)
	}
	mtype := service.function[methodName]
	if mtype == nil {
		err = errors.New("rpcx: can't find method " + methodName)
		return s.handleError(res, err)
	}

	argv := reflectTypePools.Get(mtype.ArgType)

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		err = fmt.Errorf("can not find codec for %d", req.SerializeType())
		return s.handleError(res, err)
	}

	err = codec.Decode(req.Payload, argv)
	if err != nil {
		return s.handleError(res, err)
	}

	replyv := reflectTypePools.Get(mtype.ReplyType)

	if mtype.ArgType.Kind() != reflect.Ptr {
		err = service.callForFunction(ctx, mtype, reflect.ValueOf(argv).Elem(), reflect.ValueOf(replyv))
	} else {
		err = service.callForFunction(ctx, mtype, reflect.ValueOf(argv), reflect.ValueOf(replyv))
	}

	reflectTypePools.Put(mtype.ArgType, argv)

	if err != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
		return s.handleError(res, err)
	}

	if !req.IsOneway() {
		data, err := codec.Encode(replyv)
		reflectTypePools.Put(mtype.ReplyType, replyv)
		if err != nil {
			return s.handleError(res, err)
		}
		res.Payload = data
	} else if replyv != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
	}

	return res, nil
}

func (s *Server) handleError(res *protocol.Message, err error) (*protocol.Message, error) {
	res.SetMessageStatusType(protocol.Error)
	if res.Metadata == nil {
		res.Metadata = make(map[string]string)
	}

	if s.ServerErrorFunc != nil {
		res.Metadata[protocol.ServiceError] = s.ServerErrorFunc(res, err)
	} else {
		res.Metadata[protocol.ServiceError] = err.Error()
	}

	return res, err
}

// Can connect to RPC service using HTTP CONNECT to rpcPath.
var connected = "200 Connected to rpcx"

// ServeHTTP implements an http.Handler that answers RPC requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodConnect {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Info("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")

	s.mu.Lock()
	s.activeConn[conn] = struct{}{}
	s.mu.Unlock()

	s.serveConn(conn)
}

func (s *Server) ServeWS(conn *websocket.Conn) {
	s.mu.Lock()
	s.activeConn[conn] = struct{}{}
	s.mu.Unlock()

	conn.PayloadType = websocket.BinaryFrame
	s.serveConn(conn)
}

// Close immediately closes all active net.Listeners.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.ln != nil {
		err = s.ln.Close()
	}
	for c := range s.activeConn {
		c.Close()
		delete(s.activeConn, c)
		s.Plugins.DoPostConnClose(c)
	}
	s.closeDoneChanLocked()

	if s.pool != nil {
		s.pool.StopAndWaitFor(10 * time.Second)
	}

	return err
}

// RegisterOnShutdown registers a function to call on Shutdown.
// This can be used to gracefully shutdown connections.
func (s *Server) RegisterOnShutdown(f func(s *Server)) {
	s.mu.Lock()
	s.onShutdown = append(s.onShutdown, f)
	s.mu.Unlock()
}

// RegisterOnRestart registers a function to call on Restart.
func (s *Server) RegisterOnRestart(f func(s *Server)) {
	s.mu.Lock()
	s.onRestart = append(s.onRestart, f)
	s.mu.Unlock()
}

var shutdownPollInterval = 1000 * time.Millisecond

// Shutdown gracefully shuts down the server without interrupting any
// active connections. Shutdown works by first closing the
// listener, then closing all idle connections, and then waiting
// indefinitely for connections to return to idle and then shut down.
// If the provided context expires before the shutdown is complete,
// Shutdown returns the context's error, otherwise it returns any
// error returned from closing the Server's underlying Listener.
func (s *Server) Shutdown(ctx context.Context) error {
	var err error
	if atomic.CompareAndSwapInt32(&s.inShutdown, 0, 1) {
		log.Info("shutdown begin")

		s.mu.Lock()

		// 主动注销注册的服务
		if s.Plugins != nil {
			for name := range s.serviceMap {
				s.Plugins.DoUnregister(name)
			}
		}
		if s.ln != nil {
			s.ln.Close()
		}
		for conn := range s.activeConn {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.CloseRead()
			}
		}
		s.mu.Unlock()

		// wait all in-processing requests finish.
		ticker := time.NewTicker(shutdownPollInterval)
		defer ticker.Stop()
	outer:
		for {
			if s.checkProcessMsg() {
				break
			}
			select {
			case <-ctx.Done():
				err = ctx.Err()
				break outer
			case <-ticker.C:
			}
		}

		if s.gatewayHTTPServer != nil {
			if err := s.closeHTTP1APIGateway(ctx); err != nil {
				log.Warnf("failed to close gateway: %v", err)
			} else {
				log.Info("closed gateway")
			}
		}

		if s.jsonrpcHTTPServer != nil {
			if err := s.closeJSONRPC2(ctx); err != nil {
				log.Warnf("failed to close JSONRPC: %v", err)
			} else {
				log.Info("closed JSONRPC")
			}
		}

		s.mu.Lock()
		for conn := range s.activeConn {
			conn.Close()
			delete(s.activeConn, conn)
			s.Plugins.DoPostConnClose(conn)
		}
		s.closeDoneChanLocked()

		s.mu.Unlock()

		log.Info("shutdown end")

	}
	return err
}

// Restart restarts this server gracefully.
// It starts a new rpcx server with the same port with SO_REUSEPORT socket option,
// and shutdown this rpcx server gracefully.
func (s *Server) Restart(ctx context.Context) error {
	pid, err := s.startProcess()
	if err != nil {
		return err
	}
	log.Infof("restart a new rpcx server: %d", pid)

	// TODO: is it necessary?
	time.Sleep(3 * time.Second)
	return s.Shutdown(ctx)
}

func (s *Server) startProcess() (int, error) {
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return 0, err
	}

	// Pass on the environment and replace the old count key with the new one.
	var env []string
	env = append(env, os.Environ()...)

	originalWD, _ := os.Getwd()
	allFiles := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   env,
		Files: allFiles,
	})
	if err != nil {
		return 0, err
	}
	return process.Pid, nil
}

func (s *Server) checkProcessMsg() bool {
	size := atomic.LoadInt32(&s.handlerMsgNum)
	log.Info("need handle in-processing msg size:", size)
	return size == 0
}

func (s *Server) closeDoneChanLocked() {
	select {
	case <-s.doneChan:
		// Already closed. Don't close again.
	default:
		// Safe to close here. We're the only closer, guarded
		// by s.mu.RegisterName
		close(s.doneChan)
	}
}

var ip4Reg = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	i := strings.LastIndex(ipAddress, ":")
	ipAddress = ipAddress[:i] // remove port

	return ip4Reg.MatchString(ipAddress)
}

func validIP6(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	i := strings.LastIndex(ipAddress, ":")
	ipAddress = ipAddress[:i] // remove port
	ipAddress = strings.TrimPrefix(ipAddress, "[")
	ipAddress = strings.TrimSuffix(ipAddress, "]")
	ip := net.ParseIP(ipAddress)
	if ip != nil && ip.To4() == nil {
		return true
	} else {
		return false
	}
}
