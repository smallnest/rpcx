package reflection

import (
	"context"
	"reflect"
	"testing"

	"github.com/kr/pretty"
	testutils "github.com/smallnest/rpcx/_testutils"
	"github.com/stretchr/testify/assert"
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

type Args struct {
	Aa int
	B  string
	C  bool
}

func Test_generateJSON(t *testing.T) {
	argsType := reflect.TypeOf(&Args{}).Elem()
	jsonData := generateJSON(argsType)
	assert.Equal(t, `{"Aa":0,"B":"","C":false}`, jsonData)

	def := generateTypeDefination("Args", "test", jsonData)

	result := "type Args struct {\n\tAa int64  \n\tB  string \n\tC  bool   \n}\n"
	assert.Equal(t, result, def)
}
