package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
