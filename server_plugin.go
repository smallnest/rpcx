package rpcx

import (
	"context"
	"net"

	"github.com/smallnest/rpcx/core"
)

// ServerPluginContainer implements IPluginContainer interface.
type ServerPluginContainer struct {
	plugins []IPlugin
}

// Add adds a plugin.
func (p *ServerPluginContainer) Add(plugin IPlugin) error {
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
func (p *ServerPluginContainer) Remove(pluginName string) error {
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
func (p *ServerPluginContainer) GetName(plugin IPlugin) string {
	return plugin.Name()
}

// GetByName returns a plugin instance by it's name
func (p *ServerPluginContainer) GetByName(pluginName string) IPlugin {
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
func (p *ServerPluginContainer) GetAll() []IPlugin {
	return p.plugins
}

// DoRegister invokes DoRegister plugin.
func (p *ServerPluginContainer) DoRegister(name string, rcvr interface{}, metadata ...string) error {
	var errors []error
	for i := range p.plugins {

		if plugin, ok := p.plugins[i].(IRegisterPlugin); ok {
			err := plugin.Register(name, rcvr, metadata...)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return NewMultiError(errors)
	}
	return nil
}

//DoPostConnAccept handles accepted conn
func (p *ServerPluginContainer) DoPostConnAccept(conn net.Conn) (net.Conn, bool) {
	var flag bool
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostConnAcceptPlugin); ok {
			conn, flag = plugin.HandleConnAccept(conn)
			if !flag { //interrupt
				conn.Close()
				return conn, false
			}
		}
	}
	return conn, true
}

// DoPreReadRequestHeader invokes DoPreReadRequestHeader plugin.
func (p *ServerPluginContainer) DoPreReadRequestHeader(ctx context.Context, r *core.Request) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadRequestHeaderPlugin); ok {
			err := plugin.PreReadRequestHeader(ctx, r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostReadRequestHeader invokes DoPostReadRequestHeader plugin.
func (p *ServerPluginContainer) DoPostReadRequestHeader(ctx context.Context, r *core.Request) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadRequestHeaderPlugin); ok {
			err := plugin.PostReadRequestHeader(ctx, r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreReadRequestBody invokes DoPreReadRequestBody plugin.
func (p *ServerPluginContainer) DoPreReadRequestBody(ctx context.Context, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadRequestBodyPlugin); ok {
			err := plugin.PreReadRequestBody(ctx, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostReadRequestBody invokes DoPostReadRequestBody plugin.
func (p *ServerPluginContainer) DoPostReadRequestBody(ctx context.Context, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadRequestBodyPlugin); ok {
			err := plugin.PostReadRequestBody(ctx, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPreWriteResponse invokes DoPreWriteResponse plugin.
func (p *ServerPluginContainer) DoPreWriteResponse(ctx context.Context, resp *core.Response, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreWriteResponsePlugin); ok {
			err := plugin.PreWriteResponse(ctx, resp, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DoPostWriteResponse invokes DoPostWriteResponse plugin.
func (p *ServerPluginContainer) DoPostWriteResponse(ctx context.Context, resp *core.Response, body interface{}) error {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostWriteResponsePlugin); ok {
			err := plugin.PostWriteResponse(ctx, resp, body)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type (
	//IRegisterPlugin represents register plugin.
	IRegisterPlugin interface {
		Register(name string, rcvr interface{}, metadata ...string) error
	}

	//IPostConnAcceptPlugin represents connection accept plugin.
	// if returns false, it means subsequent IPostConnAcceptPlugins should not contiune to handle this conn
	// and this conn has been closed.
	IPostConnAcceptPlugin interface {
		HandleConnAccept(net.Conn) (net.Conn, bool)
	}

	//IServerCodecPlugin represents .
	IServerCodecPlugin interface {
		IPreReadRequestHeaderPlugin
		IPostReadRequestHeaderPlugin
		IPreReadRequestBodyPlugin
		IPostReadRequestBodyPlugin
		IPreWriteResponsePlugin
		IPostWriteResponsePlugin
	}

	//IPreReadRequestHeaderPlugin represents .
	IPreReadRequestHeaderPlugin interface {
		PreReadRequestHeader(ctx context.Context, r *core.Request) error
	}

	//IPostReadRequestHeaderPlugin represents .
	IPostReadRequestHeaderPlugin interface {
		PostReadRequestHeader(ctx context.Context, r *core.Request) error
	}

	//IPreReadRequestBodyPlugin represents .
	IPreReadRequestBodyPlugin interface {
		PreReadRequestBody(ctx context.Context, body interface{}) error
	}

	//IPostReadRequestBodyPlugin represents .
	IPostReadRequestBodyPlugin interface {
		PostReadRequestBody(ctx context.Context, body interface{}) error
	}

	//IPreWriteResponsePlugin represents .
	IPreWriteResponsePlugin interface {
		PreWriteResponse(context.Context, *core.Response, interface{}) error
	}

	//IPostWriteResponsePlugin represents .
	IPostWriteResponsePlugin interface {
		PostWriteResponse(context.Context, *core.Response, interface{}) error
	}

	//IServerPluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	IServerPluginContainer interface {
		Add(plugin IPlugin) error
		Remove(pluginName string) error
		GetName(plugin IPlugin) string
		GetByName(pluginName string) IPlugin
		GetAll() []IPlugin

		DoRegister(name string, rcvr interface{}, metadata ...string) error

		DoPostConnAccept(net.Conn) (net.Conn, bool)

		DoPreReadRequestHeader(ctx context.Context, r *core.Request) error
		DoPostReadRequestHeader(ctx context.Context, r *core.Request) error

		DoPreReadRequestBody(ctx context.Context, body interface{}) error
		DoPostReadRequestBody(ctx context.Context, body interface{}) error

		DoPreWriteResponse(context.Context, *core.Response, interface{}) error
		DoPostWriteResponse(context.Context, *core.Response, interface{}) error
	}
)
