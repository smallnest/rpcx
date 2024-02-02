package example

import (
	"context"
	"github.com/smallnest/rpcx/protocol/example/pb"
	"github.com/smallnest/rpcx/server"
)

var s *server.Server

const serverAddr = "0.0.0.0:4355"

func main() {
	err := pb.RegisterHelloTestService(s, new(GreeterImpl), "httpnfo=dddjejadjflds")
	if err != nil {
		return
	}
	err = s.Serve("tcp", serverAddr)
	if err != nil {
		return
	}
}

type GreeterImpl struct{}

func (s *GreeterImpl) SayHello(ctx context.Context, args *pb.HelloRequest, reply *pb.HelloResponse) (err error) {
	*reply = pb.HelloResponse{
		Result: "test",
		Header: map[string]string{
			"naifdhaof":   "sdfjalsdfjkwe",
			"38459324":    "sdfjalsdfjkwe",
			"8345245jhfj": "sdfjalsdfjkwe",
		},
		PageInfo: map[string]int64{
			"skajdfdf444": 12,
			"77656":       98,
			"jfksadj":     1872,
		},
	}
	return nil
}
