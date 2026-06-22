package server

import (
	"context"
	"net"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/smallnest/rpcx/errors"
	"github.com/smallnest/rpcx/protocol"
	"github.com/soheilhy/cmux"
)

// PluginContainer represents a plugin container that defines all methods to manage plugins.
// And it also defines all extension points.
type PluginContainer interface {
	Add(plugin Plugin)
	Remove(plugin Plugin)
	All() []Plugin

	DoRegister(name string, rcvr interface{}, metadata string) error
	DoRegisterFunction(serviceName, fname string, fn interface{}, metadata string) error
	DoUnregister(name string) error

	DoPostConnAccept(net.Conn) (net.Conn, bool)
	DoPostConnClose(net.Conn) bool

	DoPreReadRequest(ctx context.Context) error
	DoPostReadRequest(ctx context.Context, r *protocol.Message, e error) error
	DoPostHTTPRequest(ctx context.Context, r *http.Request, params httprouter.Params) error

	DoPreHandleRequest(ctx context.Context, req *protocol.Message) error
	DoPreCall(ctx context.Context, serviceName, methodName string, args interface{}) (interface{}, error)
	DoPostCall(ctx context.Context, serviceName, methodName string, args, reply interface{}, err error) (interface{}, error)

	DoPreWriteResponse(context.Context, *protocol.Message, *protocol.Message, error) error
	DoPostWriteResponse(context.Context, *protocol.Message, *protocol.Message, error) error

	DoPreWriteRequest(ctx context.Context) error
	DoPostWriteRequest(ctx context.Context, r *protocol.Message, e error) error

	DoHeartbeatRequest(ctx context.Context, req *protocol.Message) error

	MuxMatch(m cmux.CMux)
}

// Plugin is the server plugin interface.
//
// A plugin is any value; rpcx discovers what a plugin can do by checking, at
// each extension point, whether the plugin also implements the corresponding
// XxxPlugin interface below (via a type assertion). So one plugin struct can
// implement several of these interfaces and hook into several points at once.
//
// Plugins are invoked in the order they were added with PluginContainer.Add.
// Within a single extension point, every registered plugin is called in that
// order; for the request-path hooks, if any plugin returns a non-nil error the
// container stops and returns that error immediately (the remaining plugins for
// that point are skipped). The registration hooks (DoRegister /
// DoRegisterFunction / DoUnregister) are the exception: they collect errors
// from all plugins into a MultiError instead of short-circuiting.
//
// Lifecycle order over a server's life and a single request:
//
//	On registering a service:
//	  RegisterPlugin.Register / RegisterFunctionPlugin.RegisterFunction
//
//	When a TCP-style connection is accepted:
//	  PostConnAcceptPlugin.HandleConnAccept
//
//	For each request on that connection:
//	  PreReadRequestPlugin.PreReadRequest
//	  (rpcx reads the request off the wire)
//	  PostReadRequestPlugin.PostReadRequest
//	  PreHandleRequestPlugin.PreHandleRequest
//	  PreCallPlugin.PreCall
//	  (the service method runs)
//	  PostCallPlugin.PostCall
//	  PreWriteResponsePlugin.PreWriteResponse
//	  (rpcx writes the response to the wire)
//	  PostWriteResponsePlugin.PostWriteResponse
//
//	When the connection closes:
//	  PostConnClosePlugin.HandleConnClose
//
// PreWriteRequestPlugin / PostWriteRequestPlugin wrap the server writing a
// message it initiates (for example a server-push message), not the normal
// response path above. HeartbeatPlugin.HeartbeatRequest fires instead of the
// handle path when an incoming message is a heartbeat. PostHTTPRequestPlugin
// applies to the HTTP gateway, and CMuxPlugin.MuxMatch is consulted once at
// startup when a cmux is used to multiplex protocols on one port.
type Plugin interface{}

type (
	// RegisterPlugin is invoked when a service is registered or unregistered
	// on the server (Server.Register / RegisterName / UnregisterAll).
	//
	// Register receives the service name, the receiver value being registered,
	// and its metadata string; a typical implementation publishes the service
	// to a registry such as etcd/consul/zookeeper. Unregister receives the
	// service name and removes it. Returning an error is collected into a
	// MultiError alongside other plugins' errors rather than aborting the rest.
	RegisterPlugin interface {
		Register(name string, rcvr interface{}, metadata string) error
		Unregister(name string) error
	}

	// RegisterFunctionPlugin is invoked when a bare function (not a method on a
	// service struct) is registered via Server.RegisterFunction /
	// RegisterFunctionName.
	//
	// RegisterFunction receives the service (path) name, the function name, the
	// function value, and its metadata. Errors are collected into a MultiError.
	RegisterFunctionPlugin interface {
		RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error
	}

	// PostConnAcceptPlugin is invoked right after a new connection is accepted,
	// before any request is read. It is the place to wrap or inspect the raw
	// connection (TLS bookkeeping, rate limiting, connection counting).
	//
	// HandleConnAccept receives the accepted net.Conn and returns the conn to
	// use going forward (possibly wrapped) and a bool. If the bool is false,
	// rpcx treats the connection as rejected: the container closes it and stops
	// calling subsequent PostConnAcceptPlugins for this conn.
	PostConnAcceptPlugin interface {
		HandleConnAccept(net.Conn) (net.Conn, bool)
	}

	// PostConnClosePlugin is invoked after a connection has been closed, for
	// cleanup or metrics.
	//
	// HandleConnClose receives the closed net.Conn. Returning false stops the
	// container from calling subsequent PostConnClosePlugins for this conn.
	PostConnClosePlugin interface {
		HandleConnClose(net.Conn) bool
	}

	// PreReadRequestPlugin is invoked before rpcx reads the next request
	// message from the connection.
	//
	// PreReadRequest receives the request ctx. Returning a non-nil error aborts
	// the read for this request.
	PreReadRequestPlugin interface {
		PreReadRequest(ctx context.Context) error
	}

	// PostReadRequestPlugin is invoked right after a request message has been
	// read (or a read error occurred).
	//
	// PostReadRequest receives the ctx, the decoded request message r, and the
	// read error e (nil on success). Returning a non-nil error aborts further
	// processing of this request.
	PostReadRequestPlugin interface {
		PostReadRequest(ctx context.Context, r *protocol.Message, e error) error
	}

	// PostHTTPRequestPlugin is invoked for requests arriving through the HTTP
	// gateway, after the HTTP request is read.
	//
	// PostHTTPRequest receives the ctx, the *http.Request, and the matched
	// router params. Returning a non-nil error aborts handling.
	PostHTTPRequestPlugin interface {
		PostHTTPRequest(ctx context.Context, r *http.Request, params httprouter.Params) error
	}

	// PreHandleRequestPlugin is invoked after the request is read but before
	// the service method is located and called.
	//
	// PreHandleRequest receives the ctx and the request message r. Returning a
	// non-nil error aborts handling of this request.
	PreHandleRequestPlugin interface {
		PreHandleRequest(ctx context.Context, r *protocol.Message) error
	}

	// PreCallPlugin is invoked immediately before the resolved service method
	// runs. It can inspect or replace the decoded arguments.
	//
	// PreCall receives the ctx, the service and method names, and the decoded
	// args. It returns the args to actually pass to the method (return the same
	// value to leave them unchanged) and an error; a non-nil error skips the
	// call and propagates the error.
	PreCallPlugin interface {
		PreCall(ctx context.Context, serviceName, methodName string, args interface{}) (interface{}, error)
	}

	// PostCallPlugin is invoked immediately after the service method returns.
	// It can inspect or replace the reply, or observe the method's error.
	//
	// PostCall receives the ctx, the service and method names, the args, the
	// reply produced by the method, and the method's err. It returns the reply
	// to actually send back (return the same value to leave it unchanged) and
	// an error; a non-nil error replaces the outcome for this call.
	PostCallPlugin interface {
		PostCall(ctx context.Context, serviceName, methodName string, args, reply interface{}, err error) (interface{}, error)
	}

	// PreWriteResponsePlugin is invoked just before the response is written to
	// the connection.
	//
	// PreWriteResponse receives the ctx, the request message, the response
	// message about to be written, and the handler error so far. Returning a
	// non-nil error aborts writing the response.
	PreWriteResponsePlugin interface {
		PreWriteResponse(context.Context, *protocol.Message, *protocol.Message, error) error
	}

	// PostWriteResponsePlugin is invoked right after the response has been
	// written, for metrics or cleanup.
	//
	// PostWriteResponse receives the ctx, the request message, the response
	// message that was written, and the write error (nil on success).
	PostWriteResponsePlugin interface {
		PostWriteResponse(context.Context, *protocol.Message, *protocol.Message, error) error
	}

	// PreWriteRequestPlugin is invoked before the server writes a message it
	// initiates itself (such as a server-to-client push), not the normal
	// response path.
	//
	// PreWriteRequest receives the ctx. Returning a non-nil error aborts the
	// write.
	PreWriteRequestPlugin interface {
		PreWriteRequest(ctx context.Context) error
	}

	// PostWriteRequestPlugin is invoked after such a server-initiated message
	// has been written.
	//
	// PostWriteRequest receives the ctx, the message r that was written, and
	// the write error e (nil on success).
	PostWriteRequestPlugin interface {
		PostWriteRequest(ctx context.Context, r *protocol.Message, e error) error
	}

	// HeartbeatPlugin is invoked when an incoming message is a heartbeat,
	// instead of the normal request-handling path.
	//
	// HeartbeatRequest receives the ctx and the heartbeat message req.
	// Returning a non-nil error is propagated as the heartbeat handling error.
	HeartbeatPlugin interface {
		HeartbeatRequest(ctx context.Context, req *protocol.Message) error
	}

	// CMuxPlugin lets a plugin register protocol matchers when the server uses
	// cmux to multiplex several protocols (for example HTTP and rpcx) on one
	// listening port. MuxMatch is consulted once at startup.
	CMuxPlugin interface {
		MuxMatch(m cmux.CMux)
	}
)

// pluginContainer implements PluginContainer interface.
type pluginContainer struct {
	plugins []Plugin
}

// Add adds a plugin.
func (p *pluginContainer) Add(plugin Plugin) {
	p.plugins = append(p.plugins, plugin)
}

// Remove removes a plugin by it's name.
func (p *pluginContainer) Remove(plugin Plugin) {
	if p.plugins == nil {
		return
	}

	plugins := make([]Plugin, 0, len(p.plugins))
	for _, p := range p.plugins {
		if p != plugin {
			plugins = append(plugins, p)
		}
	}

	p.plugins = plugins
}

func (p *pluginContainer) All() []Plugin {
	return p.plugins
}

// DoRegister invokes DoRegister plugin.
func (p *pluginContainer) DoRegister(name string, rcvr interface{}, metadata string) error {
	var es []error
	for _, rp := range p.plugins {
		if plugin, ok := rp.(RegisterPlugin); ok {
			err := plugin.Register(name, rcvr, metadata)
			if err != nil {
				es = append(es, err)
			}
		}
	}

	if len(es) > 0 {
		return errors.NewMultiError(es)
	}
	return nil
}

// DoRegisterFunction invokes DoRegisterFunction plugin.
func (p *pluginContainer) DoRegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	var es []error
	for _, rp := range p.plugins {
		if plugin, ok := rp.(RegisterFunctionPlugin); ok {
			err := plugin.RegisterFunction(serviceName, fname, fn, metadata)
			if err != nil {
				es = append(es, err)
			}
		}
	}

	if len(es) > 0 {
		return errors.NewMultiError(es)
	}
	return nil
}

// DoUnregister invokes RegisterPlugin.
func (p *pluginContainer) DoUnregister(name string) error {
	var es []error
	for _, rp := range p.plugins {
		if plugin, ok := rp.(RegisterPlugin); ok {
			err := plugin.Unregister(name)
			if err != nil {
				es = append(es, err)
			}
		}
	}

	if len(es) > 0 {
		return errors.NewMultiError(es)
	}
	return nil
}

// DoPostConnAccept handles accepted conn
func (p *pluginContainer) DoPostConnAccept(conn net.Conn) (net.Conn, bool) {
	var flag bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostConnAcceptPlugin); ok {
			conn, flag = plugin.HandleConnAccept(conn)
			if !flag { // interrupt
				conn.Close()
				return conn, false
			}
		}
	}
	return conn, true
}

// DoPostConnClose handles closed conn
func (p *pluginContainer) DoPostConnClose(conn net.Conn) bool {
	var flag bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostConnClosePlugin); ok {
			flag = plugin.HandleConnClose(conn)
			if !flag {
				return false
			}
		}
	}
	return true
}

// DoPreReadRequest invokes PreReadRequest plugin.
func (p *pluginContainer) DoPreReadRequest(ctx context.Context) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreReadRequestPlugin); ok {
			err := plugin.PreReadRequest(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostReadRequest invokes PostReadRequest plugin.
func (p *pluginContainer) DoPostReadRequest(ctx context.Context, r *protocol.Message, e error) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostReadRequestPlugin); ok {
			err := plugin.PostReadRequest(ctx, r, e)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostHTTPRequest invokes PostHTTPRequest plugin.
func (p *pluginContainer) DoPostHTTPRequest(ctx context.Context, r *http.Request, params httprouter.Params) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostHTTPRequestPlugin); ok {
			err := plugin.PostHTTPRequest(ctx, r, params)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreHandleRequest invokes PreHandleRequest plugin.
func (p *pluginContainer) DoPreHandleRequest(ctx context.Context, r *protocol.Message) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreHandleRequestPlugin); ok {
			err := plugin.PreHandleRequest(ctx, r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreCall invokes PreCallPlugin plugin.
func (p *pluginContainer) DoPreCall(ctx context.Context, serviceName, methodName string, args interface{}) (interface{}, error) {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreCallPlugin); ok {
			args, err = plugin.PreCall(ctx, serviceName, methodName, args)
			if err != nil {
				return args, err
			}
		}
	}

	return args, err
}

// DoPostCall invokes PostCallPlugin plugin.
func (p *pluginContainer) DoPostCall(ctx context.Context, serviceName, methodName string, args, reply interface{}, err error) (interface{}, error) {
	var e error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostCallPlugin); ok {
			reply, e = plugin.PostCall(ctx, serviceName, methodName, args, reply, err)
			if e != nil {
				return reply, e
			}
		}
	}

	return reply, e
}

// DoPreWriteResponse invokes PreWriteResponse plugin.
func (p *pluginContainer) DoPreWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreWriteResponsePlugin); ok {
			e := plugin.PreWriteResponse(ctx, req, res, err)
			if e != nil {
				return e
			}
		}
	}

	return nil
}

// DoPostWriteResponse invokes PostWriteResponse plugin.
func (p *pluginContainer) DoPostWriteResponse(ctx context.Context, req *protocol.Message, resp *protocol.Message, e error) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostWriteResponsePlugin); ok {
			err := plugin.PostWriteResponse(ctx, req, resp, e)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreWriteRequest invokes PreWriteRequest plugin.
func (p *pluginContainer) DoPreWriteRequest(ctx context.Context) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreWriteRequestPlugin); ok {
			err := plugin.PreWriteRequest(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostWriteRequest invokes PostWriteRequest plugin.
func (p *pluginContainer) DoPostWriteRequest(ctx context.Context, r *protocol.Message, e error) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostWriteRequestPlugin); ok {
			err := plugin.PostWriteRequest(ctx, r, e)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoHeartbeatRequest invokes HeartbeatRequest plugin.
func (p *pluginContainer) DoHeartbeatRequest(ctx context.Context, r *protocol.Message) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(HeartbeatPlugin); ok {
			err := plugin.HeartbeatRequest(ctx, r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// MuxMatch adds cmux Match.
func (p *pluginContainer) MuxMatch(m cmux.CMux) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(CMuxPlugin); ok {
			plugin.MuxMatch(m)
		}
	}
}
