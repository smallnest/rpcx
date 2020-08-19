package client

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/smallnest/rpcx/v5/log"
	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/share"
)

// InprocessClient is a in-process client for test.
var InprocessClient = &inprocessClient{
	services: make(map[string]interface{}),
	methods:  make(map[string]*reflect.Value),
}

// inprocessClient is a in-process client that call services in process not via TCP/UDP.
// Notice this client is only used for test.
type inprocessClient struct {
	services map[string]interface{}
	sync.RWMutex

	methods map[string]*reflect.Value
	mmu     sync.RWMutex

	ServerMessageChan chan<- *protocol.Message
}

// Connect do a fake operaton.
func (client *inprocessClient) Connect(network, address string) error {
	return nil
}

func (client *inprocessClient) RegisterServerMessageChan(ch chan<- *protocol.Message) {
	client.ServerMessageChan = ch
}

// UnregisterServerMessageChan removes ServerMessageChan.
func (client *inprocessClient) UnregisterServerMessageChan() {
	client.ServerMessageChan = nil
}

// Go calls is not async. It still use sync to call.
func (client *inprocessClient) Go(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServicePath = servicePath
	call.ServiceMethod = serviceMethod
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta != nil { //copy meta in context to meta in requests
		call.Metadata = meta.(map[string]string)
	}
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
	err := client.Call(ctx, servicePath, serviceMethod, args, reply)
	if err != nil {
		call.Error = ServiceError(err.Error())

	}
	call.done()
	return call
}

// Call calls a service synchronously.
func (client *inprocessClient) Call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}) (err error) {
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

	key := servicePath + "." + serviceMethod
	client.mmu.RLock()
	var mv = &reflect.Value{}
	mv = client.methods[key]
	client.mmu.RUnlock()

	if mv == nil {
		client.mmu.Lock()
		mv = client.methods[key]
		if mv == nil {
			v := reflect.ValueOf(service)
			t := v.MethodByName(serviceMethod)
			if t == (reflect.Value{}) {
				client.mmu.Unlock()
				return fmt.Errorf("method %s.%s not found", servicePath, serviceMethod)
			}
			mv = &t
		}
		client.mmu.Unlock()
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

func (client *inprocessClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	return nil, nil, errors.New("SendRaw method is not supported by inprocessClient")
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

func (client *inprocessClient) Unregister(name string) error {
	client.Lock()
	delete(client.services, name)
	client.Unlock()
	return nil
}
