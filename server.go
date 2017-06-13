package rpcx

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	msgpackrpc2 "github.com/rpcx-ecosystem/net-rpc-msgpackrpc2"
	"github.com/smallnest/rpcx/core"
	"github.com/smallnest/rpcx/log"
	kcp "github.com/xtaci/kcp-go"
)

const (
	//DefaultRPCPath is the defaut HTTP RPC PATH
	DefaultRPCPath = "/_goRPC_"
)

var IP4Reg = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

// ArgsContext contains net.Conn so services can get net.Conn info, for example, remote address.
type ArgsContext interface {
	Value(key string) interface{}
	SetValue(key string, value interface{})
}

type serverCodecWrapper struct {
	core.ServerCodec
	PluginContainer IServerPluginContainer
	Conn            net.Conn
	Timeout         time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

// newServerCodecWrapper wraps a core.ServerCodec.
func newServerCodecWrapper(pc IServerPluginContainer, c core.ServerCodec, Conn net.Conn) *serverCodecWrapper {
	return &serverCodecWrapper{ServerCodec: c, PluginContainer: pc, Conn: Conn}
}

func (w *serverCodecWrapper) ReadRequestHeader(ctx context.Context, r *core.Request) error {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetReadDeadline(time.Now().Add(w.ReadTimeout))
	}

	// 设置client conn in context
	if m, ok := core.FromMapContext(ctx); ok {
		m[core.ConnKey] = w.Conn
	}

	//pre
	err := w.PluginContainer.DoPreReadRequestHeader(ctx, r)
	if err != nil {
		return err
	}
	err = w.ServerCodec.ReadRequestHeader(ctx, r)

	if err != nil {
		return err
	}

	//post
	err = w.PluginContainer.DoPostReadRequestHeader(ctx, r)
	return err
}

func (w *serverCodecWrapper) ReadRequestBody(ctx context.Context, body interface{}) error {
	//pre
	err := w.PluginContainer.DoPreReadRequestBody(ctx, body)
	if err != nil {
		return err
	}
	err = w.ServerCodec.ReadRequestBody(ctx, body)
	if err != nil {
		return err
	}

	if args, ok := body.(ArgsContext); ok {
		args.SetValue("conn", w.Conn)
	}

	//post
	err = w.PluginContainer.DoPostReadRequestBody(ctx, body)
	return err
}

func (w *serverCodecWrapper) WriteResponse(ctx context.Context, resp *core.Response, body interface{}) (err error) {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetWriteDeadline(time.Now().Add(w.WriteTimeout))
	}

	// pre
	if err = w.PluginContainer.DoPreWriteResponse(ctx, resp, body); err != nil {
		return
	}

	if err = w.ServerCodec.WriteResponse(ctx, resp, body); err != nil {
		return
	}

	// post
	return w.PluginContainer.DoPostWriteResponse(ctx, resp, body)
}

func (w *serverCodecWrapper) Close() (err error) {
	//pre
	err = w.ServerCodec.Close()
	//post

	return
}

// ServerCodecFunc is used to create a core.ServerCodec from net.Conn.
type ServerCodecFunc func(conn io.ReadWriteCloser) core.ServerCodec

// Server represents a RPC Server.
type Server struct {
	ServerCodecFunc ServerCodecFunc
	//PluginContainer must be configured before starting and Register plugins must be configured before invoking RegisterName method
	PluginContainer IServerPluginContainer
	//Metadata describes extra info about this service, for example, weight, active status
	Metadata     string
	rpcServer    *core.Server
	listener     net.Listener
	Timeout      time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// use for KCP
	KCPConfig KCPConfig
}

type KCPConfig struct {
	BlockCrypt kcp.BlockCrypt
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{
		rpcServer:       core.NewServer(),
		PluginContainer: &ServerPluginContainer{plugins: make([]IPlugin, 0)},
		ServerCodecFunc: msgpackrpc2.NewServerCodec,
	}
}

// DefaultServer is the default instance of *Server.
var defaultServer = NewServer()

// Serve starts and listens RPC requests.
// It is blocked until receiving connectings from clients.
func Serve(n, address string) (err error) {
	return defaultServer.Serve(n, address)
}

// ServeTLS starts and listens RPC requests.
//It is blocked until receiving connectings from clients.
func ServeTLS(n, address string, config *tls.Config) (err error) {
	return defaultServer.ServeTLS(n, address, config)
}

// Start starts and listens RPC requests without blocking.
func Start(n, address string) (err error) {
	return defaultServer.Start(n, address)
}

// StartTLS starts and listens RPC requests without blocking.
func StartTLS(n, address string, config *tls.Config) (err error) {
	return defaultServer.StartTLS(n, address, config)
}

// ServeListener serve with a listener
func ServeListener(ln net.Listener) {
	defaultServer.ServeListener(ln)
}

// ServeByHTTP implements RPC via HTTP
func ServeByHTTP(ln net.Listener) {
	defaultServer.ServeByHTTP(ln, core.DefaultRPCPath)
}

// ServeByMux implements RPC via HTTP with customized mux
func ServeByMux(ln net.Listener, mux *http.ServeMux) {
	defaultServer.ServeByMux(ln, core.DefaultRPCPath, mux)
}

// SetServerCodecFunc sets a ServerCodecFunc
func SetServerCodecFunc(fn ServerCodecFunc) {
	defaultServer.ServerCodecFunc = fn
}

// Close closes RPC server.
func Close() error {
	return defaultServer.Close()
}

//GetListenedAddress return the listening address.
func GetListenedAddress() string {
	return defaultServer.Address()
}

// GetPluginContainer get PluginContainer of default server.
func GetPluginContainer() IServerPluginContainer {
	return defaultServer.PluginContainer
}

// RegisterName publishes in the server the set of methods .
func RegisterName(name string, service interface{}, metadata ...string) {
	defaultServer.RegisterName(name, service, metadata...)
}

// Auth sets authorization handler
func Auth(fn AuthorizationFunc) error {
	p := &AuthorizationServerPlugin{AuthorizationFunc: fn}
	return defaultServer.PluginContainer.Add(p)
}

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	i := strings.LastIndex(ipAddress, ":")
	ipAddress = ipAddress[:i] //remove port

	return IP4Reg.MatchString(ipAddress)
}

// Serve starts and listens RPC requests.
// It is blocked until receiving connectings from clients.
func (s *Server) Serve(network, address string) (err error) {
	var ln net.Listener
	ln, err = makeListener(network, address, s.KCPConfig.BlockCrypt)
	if err != nil {
		return
	}

	s.listener = ln
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}
		var ok bool
		if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
			continue
		}

		wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
		wrapper.Timeout = s.Timeout
		wrapper.ReadTimeout = s.ReadTimeout
		wrapper.WriteTimeout = s.WriteTimeout
		go s.rpcServer.ServeCodec(wrapper)
	}
}

// ServeTLS starts and listens RPC requests.
//It is blocked until receiving connectings from clients.
func (s *Server) ServeTLS(network, address string, config *tls.Config) (err error) {
	ln, err := tls.Listen(network, address, config)
	if err != nil {
		return
	}

	s.listener = ln
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}

		var ok bool
		if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
			continue
		}

		wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
		wrapper.Timeout = s.Timeout
		wrapper.ReadTimeout = s.ReadTimeout
		wrapper.WriteTimeout = s.WriteTimeout
		go s.rpcServer.ServeCodec(wrapper)
	}
}

// ServeListener starts
func (s *Server) ServeListener(ln net.Listener) {
	s.listener = ln
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}

		var ok bool
		if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
			continue
		}

		wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
		wrapper.Timeout = s.Timeout
		wrapper.ReadTimeout = s.ReadTimeout
		wrapper.WriteTimeout = s.WriteTimeout

		go s.rpcServer.ServeCodec(wrapper)
	}
}

// ServeByHTTP serves
func (s *Server) ServeByHTTP(ln net.Listener, rpcPath string) {
	http.Handle(rpcPath, s)
	srv := &http.Server{Handler: nil}
	srv.Serve(ln)
}

// ServeByMux serves
func (s *Server) ServeByMux(ln net.Listener, rpcPath string, mux *http.ServeMux) {
	mux.Handle(rpcPath, s)
	srv := &http.Server{Handler: mux}
	srv.Serve(ln)
}

var connected = "200 Connected to Go RPC"

//ServeHTTP implements net handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	c, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Errorf("rpc hijacking %s : %v", req.RemoteAddr, err.Error())
		return
	}
	io.WriteString(c, "HTTP/1.0 "+connected+"\n\n")

	var ok bool
	if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
		log.Errorf("client is not accepted: %s", c.RemoteAddr().String())
		return
	}

	wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
	wrapper.Timeout = s.Timeout
	wrapper.ReadTimeout = s.ReadTimeout
	wrapper.WriteTimeout = s.WriteTimeout

	s.rpcServer.ServeCodec(wrapper)
}

// Start starts and listens RPC requests without blocking.
func (s *Server) Start(network, address string) (err error) {
	var ln net.Listener
	ln, err = makeListener(network, address, s.KCPConfig.BlockCrypt)

	if err != nil {
		log.Errorf("failed to start server: %v", err)
		return
	}

	s.listener = ln

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}

			var ok bool
			if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
				log.Errorf("client is not accepted: %s", c.RemoteAddr().String())
				continue
			}
			wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
			wrapper.Timeout = s.Timeout
			wrapper.ReadTimeout = s.ReadTimeout
			wrapper.WriteTimeout = s.WriteTimeout

			go s.rpcServer.ServeCodec(wrapper)
		}
	}()

	return
}

// StartTLS starts and listens RPC requests without blocking.
func (s *Server) StartTLS(network, address string, config *tls.Config) (err error) {
	ln, err := tls.Listen(network, address, config)
	if err != nil {
		log.Errorf("failed to start server: %v", err)
		return
	}

	s.listener = ln

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}

			var ok bool
			if c, ok = s.PluginContainer.DoPostConnAccept(c); !ok {
				log.Errorf("client is not accepted: %s", c.RemoteAddr().String())
				continue
			}
			wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
			wrapper.Timeout = s.Timeout
			wrapper.ReadTimeout = s.ReadTimeout
			wrapper.WriteTimeout = s.WriteTimeout

			go s.rpcServer.ServeCodec(wrapper)
		}
	}()

	return
}

// Close closes RPC server.
func (s *Server) Close() error {
	return s.listener.Close()
}

// Address return the listening address.
func (s *Server) Address() string {
	return s.listener.Addr().String()
}

// RegisterName publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (s *Server) RegisterName(name string, service interface{}, metadata ...string) {
	s.rpcServer.RegisterName(name, service)
	s.PluginContainer.DoRegister(name, service, metadata...)
}

//Auth sets authorization function
func (s *Server) Auth(fn AuthorizationFunc) error {
	p := &AuthorizationServerPlugin{AuthorizationFunc: fn}
	return s.PluginContainer.Add(p)
}
