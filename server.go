package rpcx

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
)

const (
	//DefaultRPCPath is the defaut HTTP RPC PATH
	DefaultRPCPath = "/_goRPC_"
)

// ArgsContext contains net.Conn so services can get net.Conn info, for example, remote address.
type ArgsContext interface {
	Value(key string) interface{}
	SetValue(key string, value interface{})
}

type serverCodecWrapper struct {
	rpc.ServerCodec
	PluginContainer IServerPluginContainer
	Conn            net.Conn
	Timeout         time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

// newServerCodecWrapper wraps a rpc.ServerCodec.
func newServerCodecWrapper(pc IServerPluginContainer, c rpc.ServerCodec, Conn net.Conn) *serverCodecWrapper {
	return &serverCodecWrapper{ServerCodec: c, PluginContainer: pc, Conn: Conn}
}

func (w *serverCodecWrapper) ReadRequestHeader(r *rpc.Request) error {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetReadDeadline(time.Now().Add(w.ReadTimeout))
	}

	//pre
	err := w.PluginContainer.DoPreReadRequestHeader(r)
	if err != nil {
		return err
	}
	err = w.ServerCodec.ReadRequestHeader(r)

	if err != nil {
		return err
	}

	//post
	err = w.PluginContainer.DoPostReadRequestHeader(r)
	return err
}

func (w *serverCodecWrapper) ReadRequestBody(body interface{}) error {
	//pre
	err := w.PluginContainer.DoPreReadRequestBody(body)
	if err != nil {
		return err
	}

	err = w.ServerCodec.ReadRequestBody(body)
	if err != nil {
		return err
	}

	if args, ok := body.(ArgsContext); ok {
		args.SetValue("conn", w.Conn)
	}

	//post
	err = w.PluginContainer.DoPostReadRequestBody(body)
	return err
}

func (w *serverCodecWrapper) WriteResponse(resp *rpc.Response, body interface{}) (err error) {
	if w.Timeout > 0 {
		w.Conn.SetDeadline(time.Now().Add(w.Timeout))
	}
	if w.ReadTimeout > 0 {
		w.Conn.SetWriteDeadline(time.Now().Add(w.WriteTimeout))
	}

	// pre
	if err = w.PluginContainer.DoPreWriteResponse(resp, body); err != nil {
		return
	}

	if err = w.ServerCodec.WriteResponse(resp, body); err != nil {
		return
	}

	// post
	return w.PluginContainer.DoPostWriteResponse(resp, body)
}

func (w *serverCodecWrapper) Close() (err error) {
	//pre
	err = w.ServerCodec.Close()
	//post

	return
}

// ServerCodecFunc is used to create a rpc.ServerCodec from net.Conn.
type ServerCodecFunc func(conn io.ReadWriteCloser) rpc.ServerCodec

// Server represents a RPC Server.
type Server struct {
	ServerCodecFunc ServerCodecFunc
	//PluginContainer must be configured before starting and Register plugins must be configured before invoking RegisterName method
	PluginContainer IServerPluginContainer
	//Metadata describes extra info about this service, for example, weight, active status
	Metadata     string
	rpcServer    *rpc.Server
	listener     net.Listener
	Timeout      time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{
		rpcServer:       rpc.NewServer(),
		PluginContainer: &ServerPluginContainer{plugins: make([]IPlugin, 0)},
		ServerCodecFunc: msgpackrpc.NewServerCodec,
	}
}

// DefaultServer is the default instance of *Server.
var defaultServer = NewServer()

// Serve starts and listens RCP requests.
//It is blocked until receiving connectings from clients.
func Serve(n, address string) {
	defaultServer.Serve(n, address)
}

// ServeTLS starts and listens RCP requests.
//It is blocked until receiving connectings from clients.
func ServeTLS(n, address string, config *tls.Config) {
	defaultServer.ServeTLS(n, address, config)
}

// Start starts and listens RCP requests without blocking.
func Start(n, address string) {
	defaultServer.Start(n, address)
}

// StartTLS starts and listens RCP requests without blocking.
func StartTLS(n, address string, config *tls.Config) {
	defaultServer.StartTLS(n, address, config)
}

// ServeListener serve with a listener
func ServeListener(ln net.Listener) {
	defaultServer.ServeListener(ln)
}

//ServeByHTTP implements RPC via HTTP
func ServeByHTTP(ln net.Listener, rpcPath, debugPath string) {
	defaultServer.ServeByHTTP(ln, rpc.DefaultRPCPath)
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

// Serve starts and listens RCP requests.
//It is blocked until receiving connectings from clients.
func (s *Server) Serve(network, address string) {
	ln, err := net.Listen(network, address)
	if err != nil {
		return
	}

	s.listener = ln
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}

		if !s.PluginContainer.DoPostConnAccept(c) {
			continue
		}

		wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
		wrapper.Timeout = s.Timeout
		wrapper.ReadTimeout = s.ReadTimeout
		wrapper.WriteTimeout = s.WriteTimeout
		go s.rpcServer.ServeCodec(wrapper)
	}
}

// ServeTLS starts and listens RCP requests.
//It is blocked until receiving connectings from clients.
func (s *Server) ServeTLS(network, address string, config *tls.Config) {
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

		if !s.PluginContainer.DoPostConnAccept(c) {
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

		if !s.PluginContainer.DoPostConnAccept(c) {
			continue
		}

		wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
		wrapper.Timeout = s.Timeout
		wrapper.ReadTimeout = s.ReadTimeout
		wrapper.WriteTimeout = s.WriteTimeout

		go s.rpcServer.ServeCodec(wrapper)
	}
}

// ServeByHTTP starts
func (s *Server) ServeByHTTP(ln net.Listener, rpcPath string) {
	http.Handle(rpcPath, s)
	srv := &http.Server{Handler: nil}
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
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")

	if !s.PluginContainer.DoPostConnAccept(conn) {
		return
	}

	wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(conn), conn)
	wrapper.Timeout = s.Timeout
	wrapper.ReadTimeout = s.ReadTimeout
	wrapper.WriteTimeout = s.WriteTimeout

	s.rpcServer.ServeCodec(wrapper)
}

// Start starts and listens RCP requests without blocking.
func (s *Server) Start(network, address string) {
	ln, err := net.Listen(network, address)
	if err != nil {
		return
	}

	s.listener = ln

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}

			if !s.PluginContainer.DoPostConnAccept(c) {
				continue
			}
			wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
			wrapper.Timeout = s.Timeout
			wrapper.ReadTimeout = s.ReadTimeout
			wrapper.WriteTimeout = s.WriteTimeout

			go s.rpcServer.ServeCodec(wrapper)
		}
	}()
}

// StartTLS starts and listens RCP requests without blocking.
func (s *Server) StartTLS(network, address string, config *tls.Config) {
	ln, err := tls.Listen(network, address, config)
	if err != nil {
		return
	}

	s.listener = ln

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}

			if !s.PluginContainer.DoPostConnAccept(c) {
				continue
			}
			wrapper := newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c), c)
			wrapper.Timeout = s.Timeout
			wrapper.ReadTimeout = s.ReadTimeout
			wrapper.WriteTimeout = s.WriteTimeout

			go s.rpcServer.ServeCodec(wrapper)
		}
	}()
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
