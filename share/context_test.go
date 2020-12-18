package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace"
)

var (
	TheAnswer   = "Answer to the Ultimate Question of Life, the Universe, and Everything"
	MagicNumber = 42
)

func TestContext(t *testing.T) {
	rpcxContext := NewContext(context.Background())
	assert.NotNil(t, rpcxContext.Context)
	assert.NotNil(t, rpcxContext.tags)


	rpcxContext.SetValue("string", TheAnswer)
	rpcxContext.SetValue(42, MagicNumber)
	assert.Equal(t, MagicNumber, rpcxContext.Value(42))
	assert.Equal(t, TheAnswer, rpcxContext.Value("string"))


	rpcxContext.SetValue("string", TheAnswer)
	t.Log(rpcxContext.String())
}

func TestGetOpencensusSpanContextFromContext(t *testing.T) {
	var ctx Context
	ctx.SetValue("key", "value")
	ctx.SetValue(ReqMetaDataKey, make(map[string]string))

	spanCtx, err := GetOpencensusSpanContextFromContext(&ctx)
	assert.Nil(t, spanCtx)
	assert.Equal(t, "key not found", err.Error())

	PI := "3141592653589793238462643383279"

	ctx.SetValue(ReqMetaDataKey, map[string]string{
		OpencensusSpanRequestKey: PI,
	})

	spanCtx, err = GetOpencensusSpanContextFromContext(&ctx)
	var exceptTraceID [16]byte
	copy(exceptTraceID[:], []byte(PI)[:16])
	assert.Equal(t, trace.TraceID(exceptTraceID), spanCtx.TraceID)
	assert.Nil(t, err)
}

func TestWithValue(t *testing.T) {
	ctx := WithValue(context.Background(), "key", "value")
	assert.NotNil(t, ctx.tags)
}

func TestWithLocalValue(t *testing.T) {
	var c Context
	c.SetValue("key", "value")

	ctx := WithLocalValue(&c, "MagicNumber", "42")

	assert.Equal(t, "value", ctx.tags["key"])
	assert.Equal(t, "42", ctx.tags["MagicNumber"])
}