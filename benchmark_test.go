package rpcx

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/smallnest/rpcx/codec"
)

// don't use it to test benchmark. It is only used to evaluate libs internally.

func listenTCP() (net.Listener, string) {
	l, e := net.Listen("tcp", "127.0.0.1:0") // any available address
	if e != nil {
		log.Fatalf("net.Listen tcp :0: %v", e)
	}
	return l, l.Addr().String()
}

func benchmarkClient(client *rpc.Client, b *testing.B) {
	// Synchronous calls
	args := &Args{7, 8}
	procs := runtime.GOMAXPROCS(-1)
	N := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			reply := new(Reply)
			for atomic.AddInt32(&N, -1) >= 0 {
				err := client.Call("Arith.Mul", args, reply)
				if err != nil {
					b.Fatalf("rpc error: Mul: expected no error but got string %q", err.Error())
				}
				if reply.C != args.A*args.B {
					b.Fatalf("rpc error: Mul: expected %d got %d", reply.C, args.A*args.B)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	b.StopTimer()
}

func benchmarkRPCXClient(client *Client, b *testing.B) {
	// Synchronous calls
	args := &Args{7, 8}
	procs := runtime.GOMAXPROCS(-1)
	N := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			reply := new(Reply)
			for atomic.AddInt32(&N, -1) >= 0 {
				err := client.Call("Arith.Mul", args, reply)
				if err != nil {
					b.Fatalf("rpc error: Mul: expected no error but got string %q", err.Error())
				}
				if reply.C != args.A*args.B {
					b.Fatalf("rpc error: Mul: expected %d got %d", reply.C, args.A*args.B)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	b.StopTimer()
}

func benchmarkRPCXGencodeClient(client *Client, b *testing.B) {
	// Synchronous calls
	args := &GencodeArgs{7, 8}
	procs := runtime.GOMAXPROCS(-1)
	N := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			reply := new(GencodeReply)
			for atomic.AddInt32(&N, -1) >= 0 {
				err := client.Call("Arith.Mul", args, reply)
				if err != nil {
					b.Fatalf("rpc error: Mul: expected no error but got string %q", err.Error())
				}
				if reply.C != args.A*args.B {
					b.Fatalf("rpc error: Mul: expected %d got %d", reply.C, args.A*args.B)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	b.StopTimer()
}

func benchmarkRPCXProtobufClient(client *Client, b *testing.B) {
	// Synchronous calls
	args := &ProtoArgs{7, 8}
	procs := runtime.GOMAXPROCS(-1)
	N := int32(b.N)
	var wg sync.WaitGroup
	wg.Add(procs)
	b.StartTimer()

	for p := 0; p < procs; p++ {
		go func() {
			reply := new(ProtoReply)
			for atomic.AddInt32(&N, -1) >= 0 {
				err := client.Call("Arith.Mul", args, reply)
				if err != nil {
					b.Fatalf("rpc error: Mul: expected no error but got string %q", err.Error())
				}
				if reply.C != args.A*args.B {
					b.Fatalf("rpc error: Mul: expected %d got %d", reply.C, args.A*args.B)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	b.StopTimer()
}

func startNetRPCWithGob() (ln net.Listener, address string) {
	rpc.Register(new(Arith))
	ln, address = listenTCP()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Fatal("accept error:", err)
			}

			go rpc.ServeConn(conn)
		}
	}()

	return
}

func BenchmarkNetRPC_gob(b *testing.B) {
	b.StopTimer()
	_, address := startNetRPCWithGob()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal("error dialing:", err)
	}
	client := rpc.NewClient(conn)
	defer client.Close()

	benchmarkClient(client, b)
}

func startNetRPCWithJson() (ln net.Listener, address string) {
	rpc.Register(new(Arith))
	ln, address = listenTCP()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Fatal("accept error:", err)
			}

			go jsonrpc.ServeConn(conn)
		}
	}()

	return
}

func BenchmarkNetRPC_jsonrpc(b *testing.B) {
	b.StopTimer()
	_, address := startNetRPCWithJson()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal("error dialing:", err)
	}
	client := jsonrpc.NewClient(conn)
	defer client.Close()

	benchmarkClient(client, b)
}

func startNetRPCWithMsgp() (ln net.Listener, address string) {
	rpc.Register(new(Arith))
	ln, address = listenTCP()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Fatal("accept error:", err)
			}

			go msgpackrpc.ServeConn(conn)
		}
	}()

	return
}

func BenchmarkNetRPC_msgp(b *testing.B) {
	b.StopTimer()
	_, address := startNetRPCWithMsgp()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatal("error dialing:", err)
	}
	client := msgpackrpc.NewClient(conn)
	defer client.Close()

	benchmarkClient(client, b)
}

func startRPCXWithGob() *Server {
	server := NewServer()
	server.RegisterName("Arith", new(Arith))
	server.ServerCodecFunc = codec.NewGobServerCodec
	ln, _ := listenTCP()
	go server.ServeListener(ln)

	return server
}

func BenchmarkRPCX_gob(b *testing.B) {
	b.StopTimer()
	server := startRPCXWithGob()
	time.Sleep(5 * time.Second) //waiting for starting server

	s := &DirectClientSelector{Network: "tcp", Address: server.Address(), DialTimeout: 10 * time.Second}
	client := NewClient(s)
	client.ClientCodecFunc = codec.NewGobClientCodec
	defer client.Close()

	benchmarkRPCXClient(client, b)
}

func startRPCXWithJson() *Server {
	server := NewServer()
	server.RegisterName("Arith", new(Arith))
	server.ServerCodecFunc = jsonrpc.NewServerCodec
	ln, _ := listenTCP()
	go server.ServeListener(ln)

	return server
}

func BenchmarkRPCX_json(b *testing.B) {
	b.StopTimer()
	server := startRPCXWithJson()
	time.Sleep(5 * time.Second) //waiting for starting server

	s := &DirectClientSelector{Network: "tcp", Address: server.Address(), DialTimeout: 10 * time.Second}
	client := NewClient(s)
	client.ClientCodecFunc = jsonrpc.NewClientCodec
	defer client.Close()

	benchmarkRPCXClient(client, b)
}

func startRPCXWithMsgP() *Server {
	rpc.Register(new(Arith))
	server := NewServer()
	server.RegisterName("Arith", service)

	ln, _ := listenTCP()
	go server.ServeListener(ln)

	return server
}

func BenchmarkRPCX_msgp(b *testing.B) {
	b.StopTimer()
	server := startRPCXWithMsgP()
	time.Sleep(5 * time.Second) //waiting for starting server

	s := &DirectClientSelector{Network: "tcp", Address: server.Address(), DialTimeout: 10 * time.Second}
	client := NewClient(s)
	defer client.Close()

	benchmarkRPCXClient(client, b)
}

type GencodeArith int

func (t *GencodeArith) Mul(args *GencodeArgs, reply *GencodeReply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *GencodeArith) Error(args *GencodeArgs, reply *GencodeReply) error {
	panic("ERROR")
}

func startRPCXWithGencodec() *Server {
	server := NewServer()
	server.RegisterName("Arith", new(GencodeArith))
	server.ServerCodecFunc = codec.NewGencodeServerCodec
	ln, _ := listenTCP()
	go server.ServeListener(ln)

	return server
}

func BenchmarkRPCX_gencodec(b *testing.B) {
	b.StopTimer()
	server := startRPCXWithGencodec()
	time.Sleep(5 * time.Second) //waiting for starting server

	s := &DirectClientSelector{Network: "tcp", Address: server.Address(), DialTimeout: 10 * time.Second}
	client := NewClient(s)
	client.ClientCodecFunc = codec.NewGencodeClientCodec
	defer client.Close()

	benchmarkRPCXGencodeClient(client, b)
}

type ProtoArith int

func (t *ProtoArith) Mul(args *ProtoArgs, reply *ProtoReply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *ProtoArith) Error(args *ProtoArgs, reply *ProtoReply) error {
	panic("ERROR")
}

func startRPCXWithProtobuf() *Server {
	server := NewServer()
	server.RegisterName("Arith", new(ProtoArith))
	server.ServerCodecFunc = codec.NewProtobufServerCodec
	ln, _ := listenTCP()
	go server.ServeListener(ln)

	return server
}

func BenchmarkRPCX_protobuf(b *testing.B) {
	b.StopTimer()
	server := startRPCXWithProtobuf()
	time.Sleep(5 * time.Second) //waiting for starting server

	s := &DirectClientSelector{Network: "tcp", Address: server.Address(), DialTimeout: 10 * time.Second}
	client := NewClient(s)
	client.ClientCodecFunc = codec.NewProtobufClientCodec
	defer client.Close()

	benchmarkRPCXProtobufClient(client, b)
}
