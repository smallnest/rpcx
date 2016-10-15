package rpcx

import (
	"net"
	"net/rpc"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/smallnest/rpcx/codec"
)

var (
	server            *Server
	serverAddr        string
	serviceName       = "Arith/1.0"
	serviceMethodName = "Arith/1.0.Mul"
	service           = new(Arith)
	once              sync.Once
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func startServer() {
	server = NewServer()
	server.RegisterName(serviceName, service)
	server.Start("tcp", "127.0.0.1:0")
	serverAddr = server.Address()
}

func startHTTPServer() {

}

func startClient(t *testing.T) {
	conn, err := net.DialTimeout("tcp", serverAddr, time.Minute)
	if err != nil {
		t.Errorf("dialing: %v", err)
	}

	client := msgpackrpc.NewClient(conn)
	defer client.Close()

	args := &Args{7, 8}
	var reply Reply
	divCall := client.Go(serviceMethodName, args, &reply, nil)
	replyCall := <-divCall.Done // will be equal to divCall
	if replyCall.Error != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, replyCall.Error)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}
}

func startHTTPClient(t *testing.T, addr string) {
	//client, err := rpc.DialHTTPPath("tcp", addr, "foo")

	client, err := NewDirectHTTPRPCClient(nil, codec.NewGobClientCodec, "http", addr, "foo", time.Minute)
	if err != nil {
		t.Errorf("dialing: %v", err)
	}
	defer client.Close()

	args := &Args{7, 8}
	var reply Reply
	divCall := client.Go(serviceMethodName, args, &reply, nil)
	replyCall := <-divCall.Done // will be equal to divCall
	if replyCall.Error != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, replyCall.Error)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

}

func TestServe(t *testing.T) {
	once.Do(startServer)

	startClient(t)
}

func TestServeByHTTP(t *testing.T) {
	s := NewServer()
	s.ServerCodecFunc = codec.NewGobServerCodec
	s.RegisterName(serviceName, service)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go s.ServeByHTTP(ln, "foo")
	addr := ln.Addr().String()

	startHTTPClient(t, addr)
}

// Test RegisterPlugin
type logRegisterPlugin struct {
	Services map[string]interface{}
}

func (plugin *logRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) error {
	plugin.Services[name] = rcvr
	return nil
}

func (plugin *logRegisterPlugin) Name() string {
	return "logRegisterPlugin"
}

func TestRegisterPlugin(t *testing.T) {
	once.Do(startServer)

	plugin := &logRegisterPlugin{Services: make(map[string]interface{}, 1)}
	server.PluginContainer.Add(plugin)
	if _, ok := server.PluginContainer.GetByName(plugin.Name()).(IRegisterPlugin); !ok {
		t.Errorf("plugin has not been added into plugin container")
	}

	server.RegisterName("another"+serviceName, service)

	startClient(t)

	if len(plugin.Services) != 1 {
		t.Error("plugin has not been configured")
	}

	for key, value := range plugin.Services {
		if key != "another"+serviceName || value != service {
			t.Error("plugin has not been configured normally")
		}
	}
}

//Test Codec Plugin
type logCodecPlugin struct {
	preReadRequestHeader, postReadRequestHeader int
	preReadRequestBody, postReadRequestBody     int
	preWriteResponse, postWriteResponse         int
}

func (plugin *logCodecPlugin) PreReadRequestHeader(r *rpc.Request) error {
	plugin.preReadRequestHeader++
	return nil
}
func (plugin *logCodecPlugin) PostReadRequestHeader(r *rpc.Request) error {
	plugin.postReadRequestHeader++
	//fmt.Printf("Received Header: %#v\n", r)
	return nil
}
func (plugin *logCodecPlugin) PreReadRequestBody(body interface{}) error {
	plugin.preReadRequestBody++
	return nil
}
func (plugin *logCodecPlugin) PostReadRequestBody(body interface{}) error {
	//fmt.Printf("Received Body: %#v\n", body)
	plugin.postReadRequestBody++
	return nil
}
func (plugin *logCodecPlugin) PreWriteResponse(resp *rpc.Response, body interface{}) error {
	//fmt.Printf("Sent Header: %#v\nSent Body: %#v\n", resp, body)
	plugin.preWriteResponse++
	return nil
}
func (plugin *logCodecPlugin) PostWriteResponse(resp *rpc.Response, body interface{}) error {
	plugin.postWriteResponse++
	return nil
}

func (plugin *logCodecPlugin) Name() string {
	return "logCodecPlugin"
}

func TestCodecPlugin(t *testing.T) {
	once.Do(startServer)

	plugin := &logCodecPlugin{}
	server.PluginContainer.Add(plugin)

	startClient(t)

	if plugin.preReadRequestHeader == 0 ||
		plugin.postReadRequestHeader == 0 ||
		plugin.preReadRequestBody == 0 ||
		plugin.postReadRequestBody == 0 ||
		plugin.preWriteResponse == 0 ||
		plugin.postWriteResponse == 0 {
		t.Errorf("plugin has not been invoked: %v", plugin)
	}
}
