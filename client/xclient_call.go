package client

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// Core synchronous/asynchronous/raw RPC invocation for xClient
// (Go, Call, Oneshot, SendRaw). Extracted from xclient.go.

func setServerTimeout(ctx context.Context) context.Context {
	if deadline, ok := ctx.Deadline(); ok {
		metadata := ctx.Value(share.ReqMetaDataKey)
		if metadata == nil {
			metadata = map[string]string{}
			ctx = context.WithValue(ctx, share.ReqMetaDataKey, metadata)
		}
		m := metadata.(map[string]string)
		m[share.ServerTimeout] = fmt.Sprintf("%d", time.Until(deadline).Milliseconds())
	}

	return ctx
}

// Go invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
// It does not use FailMode.
func (c *xClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error) {
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

	if share.Trace {
		log.Debugf("select a client for %s.%s, args: %+v in case of xclient Go", c.servicePath, serviceMethod, args)
	}
	_, client, err := c.selectClient(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		return nil, err
	}
	if share.Trace {
		log.Debugf("selected a client %s for %s.%s, args: %+v in case of xclient Go", client.RemoteAddr(), c.servicePath, serviceMethod, args)
	}

	if done == nil {
		done = make(chan *Call, 10)
	}

	return client.Go(ctx, c.servicePath, serviceMethod, args, reply, done), nil
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// It handles errors base on FailMode.
func (c *xClient) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
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

	if share.Trace {
		log.Debugf("select a client for %s.%s, failMode: %v, args: %+v in case of xclient Call", c.servicePath, serviceMethod, c.failMode, args)
	}

	var err error
	k, client, err := c.selectClient(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		if c.failMode == Failfast || contextCanceled(err) {
			return err
		}
	}

	if share.Trace {
		if client != nil {
			log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", client.RemoteAddr(), c.servicePath, serviceMethod, c.failMode, args)
		} else {
			log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", "nil", c.servicePath, serviceMethod, c.failMode, args)
		}
	}

	var e error
	switch c.failMode {
	case Failtry:
		retries := c.option.Retries
		retryInterval := c.option.RetryInterval
		for retries >= 0 {
			retries--

			if client != nil {
				err = c.wrapCall(ctx, client, serviceMethod, args, reply)
				if err == nil {
					return nil
				}
				if contextCanceled(err) {
					return err
				}
				if e, ok := err.(ServiceError); ok && e.IsServiceError() {
					return err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			client, e = c.getCachedClient(k, c.servicePath, serviceMethod, args)
			time.Sleep(retryInterval)
		}
		if err == nil {
			err = e
		}
		return err
	case Failover:
		retries := c.option.Retries
		retryInterval := c.option.RetryInterval
		for retries >= 0 {
			retries--

			if client != nil {
				err = c.wrapCall(ctx, client, serviceMethod, args, reply)
				if err == nil {
					return nil
				}
				if contextCanceled(err) {
					return err
				}
				if e, ok := err.(ServiceError); ok && e.IsServiceError() {
					return err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			time.Sleep(retryInterval)
			// select another server
			k, client, e = c.selectClient(ctx, c.servicePath, serviceMethod, args)
		}

		if err == nil {
			err = e
		}
		return err
	case Failbackup:
		ctx, cancelFn := context.WithCancel(ctx)
		defer cancelFn()
		call1 := make(chan *Call, 10)
		call2 := make(chan *Call, 10)

		var reply1, reply2 interface{}

		if reply != nil {
			reply1 = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			reply2 = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
		}

		_, err1 := c.Go(ctx, serviceMethod, args, reply1, call1)

		t := time.NewTimer(c.option.BackupLatency)
		select {
		case <-ctx.Done(): // cancel by context
			err = ctx.Err()
			return err
		case call := <-call1:
			err = call.Error
			if err == nil && reply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply1).Elem())
			}
			return err
		case <-t.C:

		}
		_, err2 := c.Go(ctx, serviceMethod, args, reply2, call2)
		if err2 != nil {
			if uncoverError(err2) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
			err = err1
			return err
		}

		select {
		case <-ctx.Done(): // cancel by context
			err = ctx.Err()
		case call := <-call1:
			err = call.Error
			if err == nil && reply != nil && reply1 != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply1).Elem())
			}
		case call := <-call2:
			err = call.Error
			if err == nil && reply != nil && reply2 != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(reply2).Elem())
			}
		}

		return err
	default: // Failfast
		err = c.wrapCall(ctx, client, serviceMethod, args, reply)
		if err != nil {
			if uncoverError(err) {
				c.removeClient(k, c.servicePath, serviceMethod, client)
			}
		}

		return err
	}
}

// Oneshot invokes the named function, ** DOEST NOT ** wait for it to complete, and returns immediately.
func (c *xClient) Oneshot(ctx context.Context, serviceMethod string, args interface{}) error {
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

	if share.Trace {
		log.Debugf("select a client for %s.%s, args: %+v in case of xclient Go", c.servicePath, serviceMethod, args)
	}
	_, client, err := c.selectClient(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		return err
	}
	if share.Trace {
		log.Debugf("selected a client %s for %s.%s, args: %+v in case of xclient Go", client.RemoteAddr(), c.servicePath, serviceMethod, args)
	}

	client.Go(ctx, c.servicePath, serviceMethod, args, nil, nil)

	return nil
}

func uncoverError(err error) bool {
	if e, ok := err.(ServiceError); ok && e.IsServiceError() {
		return false
	}

	if err == context.DeadlineExceeded {
		return false
	}

	if err == context.Canceled {
		return false
	}

	return true
}

func contextCanceled(err error) bool {
	if err == context.DeadlineExceeded {
		return true
	}

	if err == context.Canceled {
		return true
	}

	return false
}

func (c *xClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	if c.isShutdown {
		return nil, nil, ErrXClientShutdown
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

	if share.Trace {
		log.Debugf("select a client for %s.%s, failMode: %v, args: %+v in case of xclient SendRaw", r.ServicePath, r.ServiceMethod, c.failMode, r.Payload)
	}

	var err error
	k, client, err := c.selectClient(ctx, r.ServicePath, r.ServiceMethod, r.Payload)
	if err != nil {
		if c.failMode == Failfast {
			return nil, nil, err
		}
		if contextCanceled(err) {
			return nil, nil, err
		}
		if e, ok := err.(ServiceError); ok && e.IsServiceError() {
			return nil, nil, err
		}
	}

	if share.Trace {
		log.Debugf("selected a client %s for %s.%s, failMode: %v, args: %+v in case of xclient Call", client.RemoteAddr(), r.ServicePath, r.ServiceMethod, c.failMode, r.Payload)
	}

	var e error
	switch c.failMode {
	case Failtry:
		retries := c.option.Retries
		for retries >= 0 {
			retries--
			if client != nil {
				m, payload, err := c.wrapSendRaw(ctx, client, r)
				if err == nil {
					return m, payload, nil
				}
				if contextCanceled(err) {
					return nil, nil, err
				}
				if _, ok := err.(ServiceError); ok {
					return nil, nil, err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
			client, e = c.getCachedClient(k, r.ServicePath, r.ServiceMethod, r.Payload)
		}

		if err == nil {
			err = e
		}
		return nil, nil, err
	case Failover:
		retries := c.option.Retries
		for retries >= 0 {
			retries--
			if client != nil {
				m, payload, err := c.wrapSendRaw(ctx, client, r)
				if err == nil {
					return m, payload, nil
				}
				if contextCanceled(err) {
					return nil, nil, err
				}
				if e, ok := err.(ServiceError); ok && e.IsServiceError() {
					return nil, nil, err
				}
			}

			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
			// select another server
			k, client, e = c.selectClient(ctx, r.ServicePath, r.ServiceMethod, r.Payload)
		}

		if err == nil {
			err = e
		}
		return nil, nil, err

	default: // Failfast
		m, payload, err := c.wrapSendRaw(ctx, client, r)
		if err != nil {
			if uncoverError(err) {
				c.removeClient(k, r.ServicePath, r.ServiceMethod, client)
			}
		}

		return m, payload, err
	}
}

func (c *xClient) wrapCall(ctx context.Context, client RPCClient, serviceMethod string, args interface{}, reply interface{}) error {
	if client == nil {
		return ErrServerUnavailable
	}

	if share.Trace {
		log.Debugf("call a client for %s.%s, args: %+v in case of xclient wrapCall", c.servicePath, serviceMethod, args)
	}

	if _, ok := ctx.(*share.Context); !ok {
		ctx = share.NewContext(ctx)
	}

	err := c.Plugins.DoPreCall(ctx, c.servicePath, serviceMethod, args)
	if err != nil {
		return err
	}
	err = client.Call(ctx, c.servicePath, serviceMethod, args, reply)
	c.Plugins.DoPostCall(ctx, c.servicePath, serviceMethod, args, reply, err)

	if share.Trace {
		log.Debugf("called a client for %s.%s, args: %+v, err: %v in case of xclient wrapCall", c.servicePath, serviceMethod, args, err)
	}

	return err
}

// wrapSendRaw wrap SendRaw to support client plugins
func (c *xClient) wrapSendRaw(ctx context.Context, client RPCClient, r *protocol.Message) (map[string]string, []byte, error) {
	if client == nil {
		return nil, nil, ErrServerUnavailable
	}

	if share.Trace {
		log.Debugf("call a client for %s.%s, args: %+v in case of xclient wrapSendRaw", c.servicePath, r.ServiceMethod, r.Payload)
	}

	ctx = share.NewContext(ctx)
	err := c.Plugins.DoPreCall(ctx, c.servicePath, r.ServiceMethod, r.Payload)
	if err != nil {
		return nil, nil, err
	}

	m, payload, err := client.SendRaw(ctx, r)
	c.Plugins.DoPostCall(ctx, c.servicePath, r.ServiceMethod, r.Payload, nil, err)

	if share.Trace {
		log.Debugf("called a client for %s.%s, args: %+v, err: %v in case of xclient wrapSendRaw", c.servicePath, r.ServiceMethod, r.Payload, err)
	}

	return m, payload, err
}
