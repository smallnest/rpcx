//go:build linux
// +build linux

package client

// func TestXClient_IOUring(t *testing.T) {
// 	s := server.NewServer()
// 	s.RegisterName("Arith", new(Arith), "")
// 	go s.Serve("iouring", "127.0.0.1:8972")
// 	defer s.Close()
// 	time.Sleep(500 * time.Millisecond)

// 	addr := s.Address().String()

// 	d, err := NewPeer2PeerDiscovery("iouring@"+addr, "desc=a test service")
// 	if err != nil {
// 		t.Fatalf("failed to NewPeer2PeerDiscovery: %v", err)
// 	}

// 	xclient := NewXClient("Arith", Failtry, RandomSelect, d, DefaultOption)

// 	defer xclient.Close()

// 	args := &Args{
// 		A: 10,
// 		B: 20,
// 	}

// 	reply := &Reply{}
// 	err = xclient.Call(context.Background(), "Mul", args, reply)
// 	if err != nil {
// 		t.Fatalf("failed to call: %v", err)
// 	}

// 	if reply.C != 200 {
// 		t.Fatalf("expect 200 but got %d", reply.C)
// 	}
// }
