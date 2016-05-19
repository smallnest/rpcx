package betterrpc

import (
	"net"
	"net/rpc"
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
		return ErrPluginAlreadyExists.Format(pName, p.GetDescription(plugin))
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

// GetDescription returns the name of a plugin, if no GetDescription() implemented it returns an empty string ""
func (p *ServerPluginContainer) GetDescription(plugin IPlugin) string {
	return plugin.Description()
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
func (p *ServerPluginContainer) DoRegister(name string, rcvr interface{}) error {
	var errors []error
	for i := range p.plugins {

		if plugin, ok := p.plugins[i].(IRegisterPlugin); ok {
			err := plugin.Register(name, rcvr)
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

//DoPostConnAccept handle accepted conn
func (p *ServerPluginContainer) DoPostConnAccept(conn net.Conn) bool {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostConnAcceptPlugin); ok {
			flag := plugin.Handle(conn)
			if !flag { //interrupt
				conn.Close()
				return false
			}
		}
	}
	return true
}

// DoPreReadRequestHeader invokes DoPreReadRequestHeader plugin.
func (p *ServerPluginContainer) DoPreReadRequestHeader(r *rpc.Request) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadRequestHeaderPlugin); ok {
			plugin.PreReadRequestHeader(r)
		}
	}
}

// DoPostReadRequestHeader invokes DoPostReadRequestHeader plugin.
func (p *ServerPluginContainer) DoPostReadRequestHeader(r *rpc.Request) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadRequestHeaderPlugin); ok {
			plugin.PostReadRequestHeader(r)
		}
	}
}

// DoPreReadRequestBody invokes DoPreReadRequestBody plugin.
func (p *ServerPluginContainer) DoPreReadRequestBody(body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadRequestBodyPlugin); ok {
			plugin.PreReadRequestBody(body)
		}
	}
}

// DoPostReadRequestBody invokes DoPostReadRequestBody plugin.
func (p *ServerPluginContainer) DoPostReadRequestBody(body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadRequestBodyPlugin); ok {
			plugin.PostReadRequestBody(body)
		}
	}
}

// DoPreWriteResponse invokes DoPreWriteResponse plugin.
func (p *ServerPluginContainer) DoPreWriteResponse(resp *rpc.Response, body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreWriteResponsePlugin); ok {
			plugin.PreWriteResponse(resp, body)
		}
	}
}

// DoPostWriteResponse invokes DoPostWriteResponse plugin.
func (p *ServerPluginContainer) DoPostWriteResponse(resp *rpc.Response, body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostWriteResponsePlugin); ok {
			plugin.PostWriteResponse(resp, body)
		}
	}
}

type (
	//IRegisterPlugin represents register plugin.
	IRegisterPlugin interface {
		Register(name string, rcvr interface{}) error
	}

	//IPostConnAcceptPlugin represents connection accept plugin.
	// if returns false, it means subsequent IPostConnAcceptPlugins should not contiune to handle this conn
	// and this conn has been closed.
	IPostConnAcceptPlugin interface {
		Handle(net.Conn) bool
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
		PreReadRequestHeader(r *rpc.Request)
	}

	//IPostReadRequestHeaderPlugin represents .
	IPostReadRequestHeaderPlugin interface {
		PostReadRequestHeader(r *rpc.Request)
	}

	//IPreReadRequestBodyPlugin represents .
	IPreReadRequestBodyPlugin interface {
		PreReadRequestBody(body interface{})
	}

	//IPostReadRequestBodyPlugin represents .
	IPostReadRequestBodyPlugin interface {
		PostReadRequestBody(body interface{})
	}

	//IPreWriteResponsePlugin represents .
	IPreWriteResponsePlugin interface {
		PreWriteResponse(*rpc.Response, interface{})
	}

	//IPostWriteResponsePlugin represents .
	IPostWriteResponsePlugin interface {
		PostWriteResponse(*rpc.Response, interface{})
	}

	//IServerPluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	IServerPluginContainer interface {
		Add(plugin IPlugin) error
		Remove(pluginName string) error
		GetName(plugin IPlugin) string
		GetDescription(plugin IPlugin) string
		GetByName(pluginName string) IPlugin
		GetAll() []IPlugin

		DoRegister(name string, rcvr interface{}) error

		DoPostConnAccept(net.Conn) bool

		DoPreReadRequestHeader(r *rpc.Request)
		DoPostReadRequestHeader(r *rpc.Request)

		DoPreReadRequestBody(body interface{})
		DoPostReadRequestBody(body interface{})

		DoPreWriteResponse(*rpc.Response, interface{})
		DoPostWriteResponse(*rpc.Response, interface{})
	}
)
