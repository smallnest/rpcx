package rpcx

import (
	"io"
	"net"
	"net/rpc"

	"github.com/hashicorp/net-rpc-msgpackrpc"
)

const (
	//DefaultRPCPath is the defaut HTTP RPC PATH
	DefaultRPCPath = "/_goRPC_"
)

type serverCodecWrapper struct {
	rpc.ServerCodec
	PluginContainer IServerPluginContainer
}

// newServerCodecWrapper wraps a rpc.ServerCodec.
func newServerCodecWrapper(pc IServerPluginContainer, c rpc.ServerCodec) *serverCodecWrapper {
	return &serverCodecWrapper{ServerCodec: c, PluginContainer: pc}
}

func (w *serverCodecWrapper) ReadRequestHeader(r *rpc.Request) error {

	//pre
	w.PluginContainer.DoPreReadRequestHeader(r)

	err := w.ServerCodec.ReadRequestHeader(r)

	//post
	w.PluginContainer.DoPostReadRequestHeader(r)

	return err
}

func (w *serverCodecWrapper) ReadRequestBody(body interface{}) error {
	//pre
	w.PluginContainer.DoPreReadRequestBody(body)

	err := w.ServerCodec.ReadRequestBody(body)

	//post
	w.PluginContainer.DoPostReadRequestBody(body)
	return err
}

func (w *serverCodecWrapper) WriteResponse(resp *rpc.Response, body interface{}) error {
	//pre
	w.PluginContainer.DoPreWriteResponse(resp, body)

	err := w.ServerCodec.WriteResponse(resp, body)

	//post
	w.PluginContainer.DoPostWriteResponse(resp, body)

	return err
}

func (w *serverCodecWrapper) Close() error {
	//pre
	err := w.ServerCodec.Close()
	//post

	return err
}

// ServerCodecFunc is used to create a rpc.ServerCodec from net.Conn.
type ServerCodecFunc func(conn io.ReadWriteCloser) rpc.ServerCodec

// Server represents a RPC Server.
type Server struct {
	ServerCodecFunc ServerCodecFunc
	//PluginContainer must be configured before starting and Register plugins must be configured before invoking RegisterName method
	PluginContainer IServerPluginContainer
	rpcServer       *rpc.Server
	listener        net.Listener
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

// Start starts and listens RCP requests without blocking.
func Start(n, address string) {
	defaultServer.Start(n, address)
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
func RegisterName(name string, service interface{}) {
	defaultServer.RegisterName(name, service)
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
		go s.rpcServer.ServeCodec(newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c)))
	}
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

			go s.rpcServer.ServeCodec(newServerCodecWrapper(s.PluginContainer, s.ServerCodecFunc(c)))
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
func (s *Server) RegisterName(name string, service interface{}) {
	s.rpcServer.RegisterName(name, service)
	s.PluginContainer.DoRegister(name, service)
}
