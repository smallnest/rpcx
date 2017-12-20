package client

import (
	"context"
)

// pluginContainer implements PluginContainer interface.
type pluginContainer struct {
	plugins []Plugin
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

type (
	// PreCallPlugin is invoked before the client calls a server.
	PreCallPlugin interface {
		DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error
	}

	// PostCallPlugin is invoked after the client calls a server.
	PostCallPlugin interface {
		DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error
	}

	//PluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	PluginContainer interface {
		Add(plugin Plugin)
		Remove(plugin Plugin)
		All() []Plugin

		DoPreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error
		DoPostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error
	}
)
