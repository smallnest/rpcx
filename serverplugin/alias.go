package serverplugin

import (
	"context"

	"github.com/smallnest/rpcx/v5/protocol"
)

var aliasAppliedKey = "__aliasAppliedKey"

type aliasPair struct {
	servicePath, serviceMethod string
}

//AliasPlugin can be used to set aliases for services
type AliasPlugin struct {
	Aliases          map[string]*aliasPair
	ReseverseAliases map[string]*aliasPair
}

// Alias sets a alias for the serviceMethod.
// For example Alias("anewpath&method", "Arith.mul")
func (p *AliasPlugin) Alias(aliasServicePath, aliasServiceMethod string, servicePath, serviceMethod string) {
	p.Aliases[aliasServicePath+"."+aliasServiceMethod] = &aliasPair{
		servicePath:   servicePath,
		serviceMethod: serviceMethod,
	}
	p.ReseverseAliases[servicePath+"."+serviceMethod] = &aliasPair{
		servicePath:   aliasServicePath,
		serviceMethod: aliasServiceMethod,
	}
}

// NewAliasPlugin creates a new NewAliasPlugin
func NewAliasPlugin() *AliasPlugin {
	return &AliasPlugin{
		Aliases:          make(map[string]*aliasPair),
		ReseverseAliases: make(map[string]*aliasPair),
	}
}

// PostReadRequest converts the alias of this service.
func (p *AliasPlugin) PostReadRequest(ctx context.Context, r *protocol.Message, e error) error {
	var sp = r.ServicePath
	var sm = r.ServiceMethod

	k := sp + "." + sm
	if p.Aliases != nil {
		if pm := p.Aliases[k]; pm != nil {
			r.ServicePath = pm.servicePath
			r.ServiceMethod = pm.serviceMethod
			if r.Metadata == nil {
				r.Metadata = make(map[string]string)
			}
			r.Metadata[aliasAppliedKey] = "true"
		}
	}
	return nil
}

// PreWriteResponse restore servicePath and serviceMethod.
func (p *AliasPlugin) PreWriteResponse(ctx context.Context, r *protocol.Message, res *protocol.Message) error {
	if r.Metadata[aliasAppliedKey] != "true" {
		return nil
	}
	var sp = r.ServicePath
	var sm = r.ServiceMethod

	k := sp + "." + sm
	if p.ReseverseAliases != nil {
		if pm := p.ReseverseAliases[k]; pm != nil {
			r.ServicePath = pm.servicePath
			r.ServiceMethod = pm.serviceMethod
			delete(r.Metadata, aliasAppliedKey)
			if res != nil {
				res.ServicePath = pm.servicePath
				res.ServiceMethod = pm.serviceMethod
			}
		}
	}
	return nil
}
