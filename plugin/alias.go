package plugin

import "net/rpc"

//AliasPlugin can be used to set aliases for services
type AliasPlugin struct {
	Aliases map[string]string
}

// Alias sets a alias for the serviceMethod.
// For example Alias("Arith.Mul", "mul")
func (p *AliasPlugin) Alias(alias string, serviceMethod string) {
	p.Aliases[alias] = serviceMethod
}

// NewAliasPlugin creates a new NewAliasPlugin
func NewAliasPlugin() *AliasPlugin {
	return &AliasPlugin{Aliases: make(map[string]string)}
}

// PostReadRequestHeader converts the alias of this service.
// This plugin must be added after other IPostReadRequestHeaderPlugins such AuthorizationServerPlugin,
// Because it converts the service name in requests.
func (p *AliasPlugin) PostReadRequestHeader(r *rpc.Request) error {
	var sm = r.ServiceMethod
	if p.Aliases != nil {
		if sm = p.Aliases[sm]; sm != "" {
			r.ServiceMethod = sm
		}
	}
	return nil
}

// Name return name of this plugin.
func (p *AliasPlugin) Name() string {
	return "AliasPlugin"
}
