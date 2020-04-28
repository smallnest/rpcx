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

// DoConnCreated is called in case of client connection created.
func (p *pluginContainer) DoConnCreated(conn net.Conn) (net.Conn, error) {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ConnCreatedPlugin); ok {
			conn, err = plugin.ConnCreated(conn)
			if err != nil {
				return conn, err
			}
		}
	}
	return conn, nil
}

// DoClientConnected is called in case of connected.
func (p *pluginContainer) DoClientConnected(conn net.Conn) (net.Conn, error) {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientConnectedPlugin); ok {
			conn, err = plugin.ClientConnected(conn)
			if err != nil {
				return conn, err
			}
		}
	}
	return conn, nil
}

// DoClientConnected is called in case of connected.
func (p *pluginContainer) DoClientConnectionClose(conn net.Conn) error {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientConnectionClosePlugin); ok {
			err = plugin.ClientConnectionClose(conn)
			if err != nil {
				return err
			}
		}
	}
	return err
}

// DoClientBeforeEncode is called when requests are encoded and sent.
func (p *pluginContainer) DoClientBeforeEncode(req *protocol.Message) error {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientBeforeEncodePlugin); ok {
			err = plugin.ClientBeforeEncode(req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DoClientBeforeEncode is called when requests are encoded and sent.
func (p *pluginContainer) DoClientAfterDecode(req *protocol.Message) error {
	var err error
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(ClientAfterDecodePlugin); ok {
			err = plugin.ClientAfterDecode(req)
			if err != nil {
				return err
			}
		}
	}
	return nil
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

	// ConnCreatedPlugin is invoked when the client connection has created.
	ConnCreatedPlugin interface {
		ConnCreated(net.Conn) (net.Conn, error)
	}

	// ClientConnectedPlugin is invoked when the client has connected the server.
	ClientConnectedPlugin interface {
		ClientConnected(net.Conn) (net.Conn, error)
	}

	// ClientConnectionClosePlugin is invoked when the connection is closing.
	ClientConnectionClosePlugin interface {
		ClientConnectionClose(net.Conn) error
	}

	// ClientBeforeEncodePlugin is invoked when the message is encoded and sent.
	ClientBeforeEncodePlugin interface {
		ClientBeforeEncode(*protocol.Message) error
	}

	// ClientAfterDecodePlugin is invoked when the message is decoded.
	ClientAfterDecodePlugin interface {
		ClientAfterDecode(*protocol.Message) error
	}

	//PluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	PluginContainer interface {
		Add(plugin Plugin)
		Remove(plugin Plugin)
		All() []Plugin

		DoConnCreated(net.Conn) (net.Conn, error)
		DoClientConnected(net.Conn) (net.Conn, error)
		DoClientConnectionClose(net.Conn) error

		DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error
		DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error

		DoClientBeforeEncode(*protocol.Message) error
		DoClientAfterDecode(*protocol.Message) error
	}
)
