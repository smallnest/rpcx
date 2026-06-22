package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isExported(t *testing.T) {

	assert.Equal(t, true, isExported("IsExported"))

	assert.Equal(t, false, isExported("isExported"))

	assert.Equal(t, false, isExported("_isExported"))

	assert.Equal(t, false, isExported("123_isExported"))

	assert.Equal(t, false, isExported("[]_isExported"))

	assert.Equal(t, false, isExported("&_isExported"))
}


func Mul(ctx context.Context, args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func Test_isExportedOrBuiltinType(t *testing.T) {
	typeOfMul := reflect.TypeOf(Mul)
	assert.Equal(t, true, isExportedOrBuiltinType(typeOfMul))
}

// WhitelistArith exposes two suitable RPC methods (Add, Sub), one exported
// method with an unsuitable signature (NotRPC), and one unexported method.
type WhitelistArith int

func (t *WhitelistArith) Add(ctx context.Context, args *Args, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func (t *WhitelistArith) Sub(ctx context.Context, args *Args, reply *Reply) error {
	reply.C = args.A - args.B
	return nil
}

// NotRPC is exported but is not a suitable RPC method (wrong signature).
func (t *WhitelistArith) NotRPC() string { return "not rpc" }

func TestRegisterWithMethods_subset(t *testing.T) {
	s := NewServer()
	err := s.RegisterWithMethods(new(WhitelistArith), []string{"Add"}, "")
	assert.NoError(t, err)

	svc := s.serviceMap["WhitelistArith"]
	assert.NotNil(t, svc)
	assert.Equal(t, 1, len(svc.method))
	_, hasAdd := svc.method["Add"]
	_, hasSub := svc.method["Sub"]
	assert.True(t, hasAdd)
	assert.False(t, hasSub) // Sub is suitable but not whitelisted
}

func TestRegisterWithMethods_notFound(t *testing.T) {
	s := NewServer()
	err := s.RegisterWithMethods(new(WhitelistArith), []string{"Add", "Nope"}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Nope")
	assert.Contains(t, err.Error(), "not found")
	// No partial registration.
	assert.Nil(t, s.serviceMap["WhitelistArith"])
}

func TestRegisterWithMethods_notSuitable(t *testing.T) {
	s := NewServer()
	err := s.RegisterWithMethods(new(WhitelistArith), []string{"Add", "NotRPC"}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NotRPC")
	assert.Contains(t, err.Error(), "not a suitable")
	assert.Nil(t, s.serviceMap["WhitelistArith"])
}

func TestRegisterNameWithMethods_subset(t *testing.T) {
	s := NewServer()
	err := s.RegisterNameWithMethods("Calc", new(WhitelistArith), []string{"Add", "Sub"}, "")
	assert.NoError(t, err)
	svc := s.serviceMap["Calc"]
	assert.NotNil(t, svc)
	assert.Equal(t, 2, len(svc.method))
}