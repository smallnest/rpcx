package reflection

import (
	"context"
	"testing"

	"github.com/kr/pretty"

	testutils "github.com/smallnest/rpcx/v5/_testutils"
)

type PBArith int

func (t *PBArith) Mul(ctx context.Context, args *testutils.ProtoArgs, reply *testutils.ProtoReply) error {
	reply.C = args.A * args.B
	return nil
}

func TestReflection_Register(t *testing.T) {
	r := New()
	arith := PBArith(0)
	err := r.Register("Arith", &arith, "")
	if err != nil {
		t.Fatal(err)
	}

	pretty.Println(r.Services["Arith"].String())
}
