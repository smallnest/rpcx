package server

import (
	"context"
	"github.com/smallnest/rpcx/v5/client"
	"github.com/smallnest/rpcx/v5/protocol"
	"net"
	"sync"
	"testing"
	"time"
)

type HeartbeatHandler struct{}

func (h *HeartbeatHandler) HeartbeatRequest(ctx context.Context, req *protocol.Message) error {
	conn := ctx.Value(RemoteConnContextKey).(net.Conn)
	println("OnHeartbeat:", conn.RemoteAddr().String())
	return nil
}

// TestPluginHeartbeat: go test -v -test.run TestPluginHeartbeat
func TestPluginHeartbeat(t *testing.T) {
	h := &HeartbeatHandler{}
	s := NewServer(
		WithReadTimeout(time.Duration(5)*time.Second),
		WithWriteTimeout(time.Duration(5)*time.Second),
	)
	s.Plugins.Add(h)
	s.RegisterName("Arith", new(Arith), "")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// server
		defer wg.Done()
		err := s.Serve("tcp", "127.0.0.1:9001")
		if err != nil {
			t.Log(err.Error())
		}
	}()
	go func() {
		// wait for server start complete
		time.Sleep(time.Second)
		defer wg.Done()
		// client
		opts := client.DefaultOption
		opts.Heartbeat = true
		opts.HeartbeatInterval = time.Second
		opts.ReadTimeout = time.Duration(5) * time.Second
		opts.WriteTimeout = time.Duration(5) * time.Second
		opts.ConnectTimeout = time.Duration(5) * time.Second
		// PeerDiscovery
		d := client.NewPeer2PeerDiscovery("tcp@127.0.0.1:9001", "")
		c := client.NewXClient("Arith", client.Failtry, client.RoundRobin, d, opts)
		i := 0
		for {
			i++
			resp := &Reply{}
			c.Call(context.Background(), "Mul", &Args{A: 1, B: 5}, resp)
			t.Log("call Mul resp:", resp.C)
			time.Sleep(time.Second)
			if i > 10 {
				break
			}
		}
		c.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()
	wg.Wait()
}