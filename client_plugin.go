package betterrpc

import "net/rpc"

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
		return ErrPluginAlreadyExists.Format(pName, p.GetDescription(plugin))
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

// GetDescription returns the name of a plugin, if no GetDescription() implemented it returns an empty string ""
func (p *ClientPluginContainer) GetDescription(plugin IPlugin) string {
	return plugin.Description()
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
func (p *ClientPluginContainer) DoPreReadResponseHeader(r *rpc.Response) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadResponseHeaderPlugin); ok {
			plugin.PreReadResponseHeader(r)
		}
	}
}

// DoPostReadResponseHeader invokes DoPostReadResponseHeader plugin.
func (p *ClientPluginContainer) DoPostReadResponseHeader(r *rpc.Response) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadResponseHeaderPlugin); ok {
			plugin.PostReadResponseHeader(r)
		}
	}
}

// DoPreReadResponseBody invokes DoPreReadResponseBody plugin.
func (p *ClientPluginContainer) DoPreReadResponseBody(body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreReadResponseBodyPlugin); ok {
			plugin.PreReadResponseBody(body)
		}
	}
}

// DoPostReadResponseBody invokes DoPostReadResponseBody plugin.
func (p *ClientPluginContainer) DoPostReadResponseBody(body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostReadResponseBodyPlugin); ok {
			plugin.PostReadResponseBody(body)
		}
	}
}

// DoPreWriteRequest invokes DoPreWriteRequest plugin.
func (p *ClientPluginContainer) DoPreWriteRequest(r *rpc.Request, body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPreWriteRequestPlugin); ok {
			plugin.PreWriteRequest(r, body)
		}
	}
}

// DoPostWriteRequest invokes DoPostWriteRequest plugin.
func (p *ClientPluginContainer) DoPostWriteRequest(r *rpc.Request, body interface{}) {
	for i := range p.plugins {
		if plugin, ok := p.plugins[i].(IPostWriteRequestPlugin); ok {
			plugin.PostWriteRequest(r, body)
		}
	}
}

type (

	//IPreReadResponseHeaderPlugin represents .
	IPreReadResponseHeaderPlugin interface {
		PreReadResponseHeader(*rpc.Response)
	}

	//IPostReadResponseHeaderPlugin represents .
	IPostReadResponseHeaderPlugin interface {
		PostReadResponseHeader(*rpc.Response)
	}

	//IPreReadResponseBodyPlugin represents .
	IPreReadResponseBodyPlugin interface {
		PreReadResponseBody(interface{})
	}

	//IPostReadResponseBodyPlugin represents .
	IPostReadResponseBodyPlugin interface {
		PostReadResponseBody(interface{})
	}

	//IPreWriteRequestPlugin represents .
	IPreWriteRequestPlugin interface {
		PreWriteRequest(*rpc.Request, interface{})
	}

	//IPostWriteRequestPlugin represents .
	IPostWriteRequestPlugin interface {
		PostWriteRequest(*rpc.Request, interface{})
	}

	//IClientPluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	IClientPluginContainer interface {
		Add(plugin IPlugin) error
		Remove(pluginName string) error
		GetName(plugin IPlugin) string
		GetDescription(plugin IPlugin) string
		GetByName(pluginName string) IPlugin
		GetAll() []IPlugin

		DoPreReadResponseHeader(*rpc.Response)
		DoPostReadResponseHeader(*rpc.Response)
		DoPreReadResponseBody(interface{})
		DoPostReadResponseBody(interface{})

		DoPreWriteRequest(*rpc.Request, interface{})
		DoPostWriteRequest(*rpc.Request, interface{})
	}
)
