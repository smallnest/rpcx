package client

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"

	ex "github.com/smallnest/rpcx/errors"
	"github.com/smallnest/rpcx/share"
)

// Fan-out invocation patterns for xClient: Broadcast, Fork, Inform.
// Extracted from xclient.go.

// Broadcast sends requests to all servers and Success only when all servers return OK.
// FailMode and SelectMode are meanless for this method.
// Please set timeout to avoid hanging.
func (c *xClient) Broadcast(ctx context.Context, serviceMethod string, args any, reply any) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	var replyOnce sync.Once

	ctx = setServerTimeout(ctx)
	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return ErrXClientNoServer
	}

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		go func() {
			var clonedReply any
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}

			if e == nil && reply != nil && clonedReply != nil {
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
			}
		}()
	}

check:
	for {
		select {
		case result := <-done:
			l--
			if l == 0 || !result { // all returns or some one returns an error
				break check
			}
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return err.ErrorOrNil()
}

// Fork sends requests to all servers and Success once one server returns OK.
// FailMode and SelectMode are meanless for this method.
func (c *xClient) Fork(ctx context.Context, serviceMethod string, args any, reply any) error {
	if c.isShutdown {
		return ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)

	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return ErrXClientNoServer
	}

	var replyOnce sync.Once

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		go func() {
			var clonedReply any
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			if e == nil && reply != nil && clonedReply != nil {
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
			}
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
		}()
	}

check:
	for {
		select {
		case result := <-done:
			l--
			if result {
				return nil
			}
			if l == 0 { // all returns or some one returns an error
				break check
			}
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return err.ErrorOrNil()
}

// Inform sends requests to all servers and returns all results from services.
// FailMode and SelectMode are meanless for this method.
// Please set timeout to avoid hanging.
func (c *xClient) Inform(ctx context.Context, serviceMethod string, args any, reply any) ([]Receipt, error) {
	if c.isShutdown {
		return nil, ErrXClientShutdown
	}

	if c.auth != "" {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.AuthKey] = c.auth
	}

	ctx = setServerTimeout(ctx)

	// add timeout after set server timeout, only prevent client hanging
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	callPlugins := make([]RPCClient, 0, len(c.servers))
	clients := make(map[string]RPCClient)
	c.mu.Lock()
	for k := range c.servers {
		client, needCallPlugin, err := c.getCachedClientWithoutLock(k, c.servicePath, serviceMethod)
		if err != nil {
			continue
		}
		clients[k] = client
		if needCallPlugin {
			callPlugins = append(callPlugins, client)
		}
	}
	c.mu.Unlock()

	for i := range callPlugins {
		if c.Plugins != nil {
			c.Plugins.DoClientConnected(callPlugins[i].GetConn())
		}
	}

	if len(clients) == 0 {
		return nil, ErrXClientNoServer
	}

	var receiptsLock sync.Mutex
	var receipts []Receipt

	var replyOnce sync.Once

	err := &ex.MultiError{}
	l := len(clients)
	done := make(chan bool, l)
	for k, client := range clients {
		go func() {
			var clonedReply any
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}

			e := c.wrapCall(ctx, client, serviceMethod, args, clonedReply)
			defer func() {
				done <- (e == nil)
			}()
			if e != nil {
				if uncoverError(e) {
					c.removeClient(k, c.servicePath, serviceMethod, client)
				}
				err.Append(e)
			}
			if e == nil && reply != nil && clonedReply != nil {
				replyOnce.Do(func() {
					reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				})
			}

			addr := k
			ss := strings.SplitN(k, "@", 2)
			if len(ss) == 2 {
				addr = ss[1]
			}
			receiptsLock.Lock()

			receipts = append(receipts, Receipt{
				Address: addr,
				Reply:   clonedReply,
				Error:   err,
			})
			receiptsLock.Unlock()
		}()
	}

check:
	for {
		select {
		case <-done:
			l--
			if l == 0 { // all returns or some one returns an error
				break check
			}
		}
	}

	select {
	case <-ctx.Done():
		err.Append(errors.New(("timeout")))
	default:
	}

	return receipts, err.ErrorOrNil()
}
