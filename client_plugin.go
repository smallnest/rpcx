package rpcx

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/core"
)

// ClientPluginContainer implements IPluginContainer interface.
type ClientPluginContainer struct {
	plugins []IPlugin
}

// Add adds a plugin.
func (p *ClientPluginContainer) Add(plugin IPlugin) error {
	if p.plugins == nil {
		p.plugins = make([]IPlugin, 0)
	}

	pName := p.GetName(plugin)
	if pName != "" && p.GetByName(pName) != nil {
		return ErrPluginAlreadyExists.Format(pName)
	}

	p.plugins = append(p.plugins, plugin)
	return nil
}

// Remove removes a plugin by it's name.
func (p *ClientPluginContainer) Remove(pluginName string) error {
	if p.plugins == nil {
		return ErrPluginRemoveNoPlugins.Return()
	}

	if pluginName == "" {
		//return error: cannot delete an unamed plugin
		return ErrPluginRemoveEmptyName.Return()
	}

	indexToRemove := -1
	for i := range p.plugins {
		if p.GetName(p.plugins[i]) == pluginName {
			indexToRemove = i
			break
		}
	}
	if indexToRemove == -1 {
		return ErrPluginRemoveNotFound.Return()
	}

	p.plugins = append(p.plugins[:indexToRemove], p.plugins[indexToRemove+1:]...)

	return nil
}

// GetName returns the name of a plugin, if no GetName() implemented it returns an empty string ""
func (p *ClientPluginContainer) GetName(plugin IPlugin) string {
	return plugin.Name()
}

// GetByName returns a plugin instance by it's name
func (p *ClientPluginContainer) GetByName(pluginName string) IPlugin {
	if p.plugins == nil {
		return nil
	}

	for _, plugin := range p.plugins {
		if plugin.Name() == pluginName {
			return plugin
		}
	}

	return nil
}

// GetAll returns all activated plugins
func (p *ClientPluginContainer) GetAll() []IPlugin {
	return p.plugins
}

// DoPreReadResponseHeader invokes DoPreReadResponseHeader plugin.
func (p *ClientPluginContainer) DoPreReadResponseHeader(r *core.Response) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadResponseHeaderPlugin); ok {
			err := plugin.PreReadResponseHeader(r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DoPostReadResponseHeader invokes DoPostReadResponseHeader plugin.
func (p *ClientPluginContainer) DoPostReadResponseHeader(r *core.Response) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadResponseHeaderPlugin); ok {
			err := plugin.PostReadResponseHeader(r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DoPreReadResponseBody invokes DoPreReadResponseBody plugin.
func (p *ClientPluginContainer) DoPreReadResponseBody(body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadResponseBodyPlugin); ok {
			err := plugin.PreReadResponseBody(body)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DoPostReadResponseBody invokes DoPostReadResponseBody plugin.
func (p *ClientPluginContainer) DoPostReadResponseBody(body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadResponseBodyPlugin); ok {
			err := plugin.PostReadResponseBody(body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreWriteRequest invokes DoPreWriteRequest plugin.
func (p *ClientPluginContainer) DoPreWriteRequest(ctx context.Context, r *core.Request, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreWriteRequestPlugin); ok {
			err := plugin.PreWriteRequest(ctx, r, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostWriteRequest invokes DoPostWriteRequest plugin.
func (p *ClientPluginContainer) DoPostWriteRequest(ctx context.Context, r *core.Request, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostWriteRequestPlugin); ok {
			err := plugin.PostWriteRequest(ctx, r, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

//DoPostConnected handles connected
func (p *ClientPluginContainer) DoPostConnected(conn net.Conn) (net.Conn, bool) {
	var flag bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostConnectedPlugin); ok {
			conn, flag = plugin.HandleConnected(conn)
			if !flag { //interrupt
				conn.Close()
				return conn, false
			}
		}
	}
	return conn, true
}

// DoPreCall executes before call
func (p *ClientPluginContainer) DoPreCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreCallPlugin); ok {
			err := plugin.DoPreCall(ctx, serviceMethod, args, reply)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostCall executes after call
func (p *ClientPluginContainer) DoPostCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostCallPlugin); ok {
			err := plugin.DoPostCall(ctx, serviceMethod, args, reply)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type (
	//IPostConnectedPlugin represents connected plugin.
	IPostConnectedPlugin interface {
		HandleConnected(net.Conn) (net.Conn, bool)
	}

	//IPreReadResponseHeaderPlugin represents .
	IPreReadResponseHeaderPlugin interface {
		PreReadResponseHeader(*core.Response) error
	}

	//IPostReadResponseHeaderPlugin represents .
	IPostReadResponseHeaderPlugin interface {
		PostReadResponseHeader(*core.Response) error
	}

	//IPreReadResponseBodyPlugin represents .
	IPreReadResponseBodyPlugin interface {
		PreReadResponseBody(interface{}) error
	}

	//IPostReadResponseBodyPlugin represents .
	IPostReadResponseBodyPlugin interface {
		PostReadResponseBody(interface{}) error
	}

	//IPreWriteRequestPlugin represents .
	IPreWriteRequestPlugin interface {
		PreWriteRequest(context.Context, *core.Request, interface{}) error
	}

	//IPostWriteRequestPlugin represents .
	IPostWriteRequestPlugin interface {
		PostWriteRequest(context.Context, *core.Request, interface{}) error
	}

	// IPreCallPlugin is invoked before the client calls a server.
	IPreCallPlugin interface {
		DoPreCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	}

	// IPostCallPlugin is invoked after the client calls a server.
	IPostCallPlugin interface {
		DoPostCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	}

	//IClientPluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	IClientPluginContainer interface {
		Add(plugin IPlugin) error
		Remove(pluginName string) error
		GetName(plugin IPlugin) string
		GetByName(pluginName string) IPlugin
		GetAll() []IPlugin

		DoPostConnected(net.Conn) (net.Conn, bool)

		DoPreReadResponseHeader(*core.Response) error
		DoPostReadResponseHeader(*core.Response) error
		DoPreReadResponseBody(interface{}) error
		DoPostReadResponseBody(interface{}) error

		DoPreWriteRequest(context.Context, *core.Request, interface{}) error
		DoPostWriteRequest(context.Context, *core.Request, interface{}) error

		DoPreCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
		DoPostCall(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error
	}
)
