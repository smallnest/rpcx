package plugin

import (
	"bytes"
	"encoding/gob"
	"net/rpc"
)

// AuthorizationServerPlugin is used to authorize clients.
type AuthorizationServerPlugin struct {
	AuthorizationFunc AuthorizationFunc
}

// AuthorizationFunc defines a method type which handles Authorization info
type AuthorizationFunc func(p *AuthorizationAndServiceMethod) error

// AuthorizationAndServiceMethod represents Authorization header and ServiceMethod.
type AuthorizationAndServiceMethod struct {
	Authorization string // Authorization
	ServiceMethod string // real ServiceMethod name
	Tag           string // extra tag for Authorization
}

func init() {
	// This type must match exactly what youre going to be using,
	// down to whether or not its a pointer
	gob.Register(&AuthorizationAndServiceMethod{})
}

// PostReadRequestHeader extracts Authorization header from ServiceMethod field.
func (plugin *AuthorizationServerPlugin) PostReadRequestHeader(r *rpc.Request) (err error) {
	b := bytes.NewBufferString(r.ServiceMethod)
	var aAndS AuthorizationAndServiceMethod
	dec := gob.NewDecoder(b)
	err = dec.Decode(&aAndS)
	if err == nil {
		r.ServiceMethod = aAndS.ServiceMethod

		if plugin.AuthorizationFunc != nil {
			err = plugin.AuthorizationFunc(&aAndS)
		}
	}
	return
}

// Name return name of this plugin.
func (plugin *AuthorizationServerPlugin) Name() string {
	return "AuthorizationServerPlugin"
}

// Description return description of this plugin.
func (plugin *AuthorizationServerPlugin) Description() string {
	return "a Authorization plugin"
}

// AuthorizationClientPlugin is used to set Authorization info at client side.
type AuthorizationClientPlugin struct {
	AuthorizationAndServiceMethod *AuthorizationAndServiceMethod
}

// NewAuthorizationClientPlugin creates a AuthorizationClientPlugin with authorization header and tag
func NewAuthorizationClientPlugin(authorization, tag string) *AuthorizationClientPlugin {
	return &AuthorizationClientPlugin{
		AuthorizationAndServiceMethod: &AuthorizationAndServiceMethod{
			Authorization: authorization,
			Tag:           tag,
		},
	}
}

// PreWriteRequest adds Authorization info in requests
func (plugin *AuthorizationClientPlugin) PreWriteRequest(r *rpc.Request, body interface{}) error {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	plugin.AuthorizationAndServiceMethod.ServiceMethod = r.ServiceMethod
	err := enc.Encode(plugin.AuthorizationAndServiceMethod)
	if err != nil {
		return err
	}

	r.ServiceMethod = b.String()
	return nil
}

// Name return name of this plugin.
func (plugin *AuthorizationClientPlugin) Name() string {
	return "AuthorizationClientPlugin"
}

// Description return description of this plugin.
func (plugin *AuthorizationClientPlugin) Description() string {
	return "a Authorization plugin"
}
