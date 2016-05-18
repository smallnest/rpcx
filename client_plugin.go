package betterrpc

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

type (

	//IClientPluginContainer represents a plugin container that defines all methods to manage plugins.
	//And it also defines all extension points.
	IClientPluginContainer interface {
		Add(plugin IPlugin) error
		Remove(pluginName string) error
		GetName(plugin IPlugin) string
		GetDescription(plugin IPlugin) string
		GetByName(pluginName string) IPlugin
		GetAll() []IPlugin
	}
)
