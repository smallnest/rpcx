package client

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/smallnest/rpcx/log"
)

// InprocessClient is a in-process client for test.
var InprocessClient = &inprocessClient{
	services: make(map[string]interface{}),
}

// inprocessClient is a in-process client that call services in process not via TCP/UDP.
// Notice this client is only used for test.
type inprocessClient struct {
	services map[string]interface{}
	sync.RWMutex
}

// Connect do a fake operaton.
func (client *inprocessClient) Connect(network, address string) error {
	return nil
}

// Go calls is not async. It still use sync to call.
func (client *inprocessClient) Go(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, metadata map[string]string, done chan *Call) *Call {
	call := new(Call)
	call.ServicePath = servicePath
	call.ServiceMethod = serviceMethod
	call.Metadata = metadata
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel. If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("rpc: done channel is unbuffered")
		}
	}
	call.Done = done
	err := client.Call(ctx, servicePath, serviceMethod, args, reply, metadata)
	if err != nil {
		call.Error = ServiceError(err.Error())

	}
	call.done()
	return call
}

// Call calls a service synchronously.
func (client *inprocessClient) Call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, metadata map[string]string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			var ok bool
			if err, ok = e.(error); ok {
				err = fmt.Errorf("failed to call %s.%s because of %v", servicePath, serviceMethod, err)
			}
		}
	}()

	client.RLock()
	service := client.services[servicePath]
	client.RUnlock()
	if service == nil {
		return fmt.Errorf("service %s not found", servicePath)
	}

	v := reflect.ValueOf(service)
	mv := v.MethodByName(serviceMethod)
	if mv == (reflect.Value{}) {
		return fmt.Errorf("method %s.%s not found", servicePath, serviceMethod)
	}

	argv := reflect.ValueOf(args)
	replyv := reflect.ValueOf(reply)

	err = nil
	returnValues := mv.Call([]reflect.Value{reflect.ValueOf(ctx), argv, replyv})
	errInter := returnValues[0].Interface()
	if errInter != nil {
		err = errInter.(error)
	}

	return err
}

// Close do a fake operaton.
func (client *inprocessClient) Close() error {
	return nil
}

// IsClosing always returns false.
func (client *inprocessClient) IsClosing() bool {
	return false
}

// IsShutdown always return false.
func (client *inprocessClient) IsShutdown() bool {
	return false
}

func (client *inprocessClient) Register(name string, rcvr interface{}, metadata string) (err error) {
	client.Lock()
	client.services[name] = rcvr
	client.Unlock()
	return
}
