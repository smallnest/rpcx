package main

import (
	"sync"
	"time"

	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/clientselector"
	"github.com/smallnest/rpcx/log"
)

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

func main() {
	server1 := &clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8972"}
	server2 := &clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8973"}

	servers := []*clientselector.ServerPeer{server1, server2}

	s := clientselector.NewMultiClientSelector(servers, rpcx.RandomSelect, 10*time.Second)

	clientPool := &pool{
		max: 100,
		New: func() *rpcx.Client {
			return rpcx.NewClient(s)
		},
	}

	var sg sync.WaitGroup
	sg.Add(1000)
	for i := 0; i < 1000; i++ {
		go callServer(sg, clientPool, s)
		time.Sleep(10 * time.Millisecond)
	}

	sg.Wait()
	clientPool.Close()

}

func callServer(sg sync.WaitGroup, clientPool *pool, s rpcx.ClientSelector) {
	client := clientPool.Get()

	args := &Args{7, 8}
	var reply Reply
	err := client.Call("Arith.Mul", args, &reply)
	if err != nil {
		log.Infof("error for Arith: %d*%d, %v", args.A, args.B, err)
	} else {
		log.Infof("Arith: %d*%d=%d, client: %p", args.A, args.B, reply.C, client)
	}

	clientPool.Put(client)
	sg.Done()
}

// client Pool

type pool struct {
	max     int
	clients []*rpcx.Client
	sync.Mutex
	size int
	New  func() *rpcx.Client
}

func (p *pool) Get() (c *rpcx.Client) {
	p.Lock()
	defer p.Unlock()

	//always return a client
	if p.size < 1 {
		return p.New()
	}

	c = p.clients[0]
	p.clients = p.clients[1:]
	p.size--
	return c
}

func (p *pool) Put(c *rpcx.Client) {
	p.Lock()
	defer p.Unlock()

	if p.size >= p.max {
		c.Close()
		return
	}

	p.clients = append(p.clients, c)
	p.size++
}

func (p *pool) Close() {
	p.Lock()
	defer p.Unlock()

	for _, c := range p.clients {
		c.Close()
	}

	p.clients = p.clients[:0]
	p.size = 0
}
