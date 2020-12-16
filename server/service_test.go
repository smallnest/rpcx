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