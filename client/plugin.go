package client

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/protocol"
)

// pluginContainer implements PluginContainer interface.
type pluginContainer struct {
	plugins []Plugin
}

func NewPluginContainer() PluginContainer {
	return &pluginContainer{}
}

// Plugin is the client plugin interface.
type Plugin interface {
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

	var plugins []Plugin
	for _, pp := range p.plugins {
		if pp != plugin {
			plugins = append(plugins, pp)
		}
	}

	p.plugins = plugins
}

// All returns all plugins
func (p *pluginContainer) All() []Plugin {
	return p.plugins
}

// DoPreCall executes before call
func (p *pluginContainer) DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PreCallPlugin); ok {
			err := plugin.DoPreCall(ctx, servicePath, serviceMethod, args)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostCall executes after call
func (p *pluginContainer) DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(PostCallPlugin); ok {
			err = plugin.DoPostCall(ctx, servicePath, serviceMethod, args, reply, err)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DoClientConnected is called in case of connected.
func (p *pluginContainer) DoClientConnected(conn net.Conn) (net.Conn, bool) {
	var handleOk bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientConnectedPlugin); ok {
			conn, handleOk = plugin.ClientConnected(conn)
			if !handleOk {
				return conn, false
			}
		}
	}
	return conn, true
}

// DoClientConnected is called in case of connected.
func (p *pluginContainer) DoClientConnectionClose(conn net.Conn) bool {
	var handleOk bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientConnectionClosePlugin); ok {
			handleOk = plugin.ClientConnectionClose(conn)
			if !handleOk {
				return false
			}
		}
	}
	return true
}

// DoClientBeforeEncode is called when requests are encoded and sent.
func (p *pluginContainer) DoClientBeforeEncode(req *protocol.Message) bool {
	var handleOk bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientBeforeEncodePlugin); ok {
			handleOk = plugin.ClientBeforeEncode(req)
			if !handleOk {
				return false
			}
		}
	}
	return true
}

// DoClientBeforeEncode is called when requests are encoded and sent.
func (p *pluginContainer) DoClientAfterDecode(req *protocol.Message) bool {
	var handleOk bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientAfterDecodePlugin); ok {
			handleOk = plugin.ClientAfterDecode(req)
			if !handleOk {
				return false
			}
		}
	}
	return true
}

type (
	// PreCallPlugin is invoked before the client calls a server.
	PreCallPlugin interface {
		DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error
	}

	// PostCallPlugin is invoked after the client calls a server.
	PostCallPlugin interface {
		DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error
	}

	// ClientConnectedPlugin is invoked when the client has connected the server.
	ClientConnectedPlugin interface {
		ClientConnected(net.Conn) (net.Conn, bool)
	}

	// ClientConnectionClosePlugin is invoked when the connection is closing.
	ClientConnectionClosePlugin interface {
		ClientConnectionClose(net.Conn) bool
	}

	// ClientBeforeEncodePlugin is invoked when the message is encoded and sent.
	ClientBeforeEncodePlugin interface {
		ClientBeforeEncode(*protocol.Message) bool
	}

	// ClientAfterDecodePlugin is invoked when the message is decoded.
	ClientAfterDecodePlugin interface {
		ClientAfterDecode(*protocol.Message) bool
	}

	//PluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	PluginContainer interface {
		Add(plugin Plugin)
		Remove(plugin Plugin)
		All() []Plugin

		DoClientConnected(net.Conn) (net.Conn, bool)
		DoClientConnectionClose(net.Conn) bool

		DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error
		DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error

		DoClientBeforeEncode(*protocol.Message) bool
		DoClientAfterDecode(*protocol.Message) bool
	}
)
