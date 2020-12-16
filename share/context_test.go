package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)


var (
	TheAnswer = "Answer to the Ultimate Question of Life, the Universe, and Everything"
	MagicNumber = 42
)

func TestNewContext(t *testing.T) {
	rpcxContext := NewContext(context.Background())

	assert.NotNil(t, rpcxContext.Context)

	assert.NotNil(t, rpcxContext.tags)
}

func TestContext_SetValue(t *testing.T) {
	rpcxContext := NewContext(context.Background())

	rpcxContext.SetValue("string", TheAnswer)
	rpcxContext.SetValue(42, MagicNumber)

	assert.Equal(t, MagicNumber, rpcxContext.Value(42))

	assert.Equal(t, TheAnswer, rpcxContext.Value("string"))
}

func TestContext_String(t *testing.T) {
	rpcxContext := NewContext(context.Background())

	rpcxContext.SetValue("string", TheAnswer)
	t.Log(rpcxContext.String())
}

func TestWithValue(t *testing.T) {

}