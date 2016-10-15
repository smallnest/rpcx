package rpcx

import (
	"encoding/gob"
	"errors"
	"net/rpc"
	"strings"
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

func (aasm *AuthorizationAndServiceMethod) String() string {
	return aasm.ServiceMethod + "\x1f" + aasm.Authorization + "\x1f" + aasm.Tag
}

func init() {
	// This type must match exactly what youre going to be using,
	// down to whether or not its a pointer
	gob.Register(&AuthorizationAndServiceMethod{})
}

// PostReadRequestHeader extracts Authorization header from ServiceMethod field.
func (plugin *AuthorizationServerPlugin) PostReadRequestHeader(r *rpc.Request) (err error) {
	items := strings.Split(r.ServiceMethod, "\x1f")

	if len(items) != 3 {
		return errors.New("wrong authorization format for " + r.ServiceMethod)
	}

	aAndS := &AuthorizationAndServiceMethod{ServiceMethod: items[0], Authorization: items[1], Tag: items[2]}

	r.ServiceMethod = aAndS.ServiceMethod

	if plugin.AuthorizationFunc != nil {
		err = plugin.AuthorizationFunc(aAndS)
	}
	return err
}

// Name return name of this plugin.
func (plugin *AuthorizationServerPlugin) Name() string {
	return "AuthorizationServerPlugin"
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
	plugin.AuthorizationAndServiceMethod.ServiceMethod = r.ServiceMethod

	r.ServiceMethod = plugin.AuthorizationAndServiceMethod.String()
	return nil
}

// Name return name of this plugin.
func (plugin *AuthorizationClientPlugin) Name() string {
	return "AuthorizationClientPlugin"
}
