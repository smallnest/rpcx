package server

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
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
	Stop() context.Context
	StopAndWait()
}

// Server is rpcx server that use TCP or UDP.
type Server struct {
	ln                net.Listener
	readTimeout       time.Duration
	writeTimeout      time.Duration
	gatewayHTTPServer *http.Server

	jsonrpcHTTPServerLock sync.Mutex
	jsonrpcHTTPServer     *http.Server
	DisableHTTPGateway    bool // disable http invoke or not.
	DisableJSONRPC        bool // disable json rpc or not.
	AsyncWrite            bool // set true if your server only serves few clients
	pool                  WorkerPool

	serviceMapMu sync.RWMutex
	serviceMap   map[string]*service

	router map[string]Handler

	mu         sync.RWMutex
	activeConn map[net.Conn]struct{}
	doneChan   chan struct{}
	seq        atomic.Uint64

	inShutdown int32
	onShutdown []func(s *Server)
	onRestart  []func(s *Server)

	// TLSConfig for creating tls tcp connection.
	tlsConfig *tls.Config
	// BlockCrypt for kcp.BlockCrypt
	options map[string]any
	// CORS options
	corsOptions *CORSOptions

	Plugins PluginContainer

	// AuthFunc can be used to auth.
	AuthFunc func(ctx context.Context, req *protocol.Message, token string) error

	handlerMsgNum int32
	requestCount  atomic.Uint64

	// HandleServiceError is used to get all service errors. You can use it write logs or others.
	HandleServiceError func(error)

	// ServerErrorFunc is a customized error handlers and you can use it to return customized error strings to clients.
	// If not set, it use err.Error()
	ServerErrorFunc func(res *protocol.Message, err error) string

	// The server is started.
	Started           chan struct{}
	unregisterAllOnce sync.Once
}

// NewServer returns a server.
func NewServer(options ...OptionFn) *Server {
	s := &Server{
		Plugins:    &pluginContainer{},
		options:    make(map[string]any),
		activeConn: make(map[net.Conn]struct{}),
		doneChan:   make(chan struct{}),
		serviceMap: make(map[string]*service),
		router:     make(map[string]Handler),
		AsyncWrite: false, // 除非你想做进一步的优化测试，否则建议你设置为false
		Started:    make(chan struct{}),
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

// Serve starts and listens RPC requests.
// It is blocked until receiving connections from clients.
func (s *Server) Serve(network, address string) (err error) {
	var ln net.Listener
	ln, err = s.makeListener(network, address)
	if err != nil {
		return err
	}

	defer s.UnregisterAll()

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

// ServeListener listens RPC requests.
// It is blocked until receiving connections from clients.
func (s *Server) ServeListener(network string, ln net.Listener) (err error) {
	defer s.UnregisterAll()

	if network == "http" {
		s.serveByHTTP(ln, "")
		return nil
	}

	// try to start gateway
	ln = s.startGateway(network, ln)

	return s.serveListener(ln)
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
