package server

import (
	"context"
	"testing"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// benchArith is a minimal service whose arg/reply types implement Reset so the
// server object pool actually reuses them (mirroring the intended hot path).
// It reuses the exported Args/Reply from server_test.go via BenchArgs/BenchReply
// aliases that add Reset; the types must be exported to be suitable RPC methods.
type benchArith int

type BenchArgs struct {
	A int
	B int
}

func (a *BenchArgs) Reset() { a.A, a.B = 0, 0 }

type BenchReply struct {
	C int
}

func (r *BenchReply) Reset() { r.C = 0 }

func (t *benchArith) Mul(ctx context.Context, args *BenchArgs, reply *BenchReply) error {
	reply.C = args.A * args.B
	return nil
}

// newBenchRequest builds a JSON-encoded Mul request message.
func newBenchRequest(b *testing.B) *protocol.Message {
	b.Helper()
	codec := share.Codecs[protocol.JSON]
	payload, err := codec.Encode(&BenchArgs{A: 7, B: 6})
	if err != nil {
		b.Fatalf("encode payload: %v", err)
	}
	req := protocol.NewMessage()
	req.SetMessageType(protocol.Request)
	req.SetSerializeType(protocol.JSON)
	req.ServicePath = "benchArith"
	req.ServiceMethod = "Mul"
	req.Payload = payload
	return req
}

// BenchmarkServerHandleRequest exercises the per-request dispatch path
// (service lookup, arg/reply pool Get/Put, decode, call, encode). It is the
// benchmark used to validate object-pool changes in server_dispatch.go.
func BenchmarkServerHandleRequest(b *testing.B) {
	s := NewServer()
	if err := s.RegisterName("benchArith", new(benchArith), ""); err != nil {
		b.Fatalf("register: %v", err)
	}
	req := newBenchRequest(b)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := s.handleRequest(ctx, req)
		if err != nil {
			b.Fatalf("handleRequest: %v", err)
		}
		_ = res
	}
}
