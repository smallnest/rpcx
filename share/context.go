package share

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// var _ context.Context = &Context{}

// Context is a rpcx customized Context that can contains multiple values.
type Context struct {
	tagsLock *sync.Mutex
	tags     map[any]any
	context.Context
}

func NewContext(ctx context.Context) *Context {
	tagsLock := &sync.Mutex{}
	ctx = context.WithValue(ctx, ContextTagsLock, tagsLock)
	return &Context{
		tagsLock: tagsLock,
		Context:  ctx,
		tags:     map[any]any{isShareContext: true},
	}
}

func (c *Context) Lock() {
	c.tagsLock.Lock()
}

func (c *Context) Unlock() {
	c.tagsLock.Unlock()
}

func (c *Context) Value(key any) any {
	c.tagsLock.Lock()
	defer c.tagsLock.Unlock()
	if c.tags == nil {
		c.tags = make(map[any]any)
	}

	if v, ok := c.tags[key]; ok {
		return v
	}
	return c.Context.Value(key)
}

func (c *Context) SetValue(key, val any) {
	c.tagsLock.Lock()
	defer c.tagsLock.Unlock()

	if c.tags == nil {
		c.tags = make(map[any]any)
	}
	c.tags[key] = val
}

// DeleteKey delete the kv pair by key.
func (c *Context) DeleteKey(key any) {
	c.tagsLock.Lock()
	defer c.tagsLock.Unlock()

	if c.tags == nil || key == nil {
		return
	}
	delete(c.tags, key)
}

func (c *Context) String() string {
	return fmt.Sprintf("%v.WithValue(%v)", c.Context, c.tags)
}

// WithValue returns a new share Context derived from parent that carries the
// key/val pair in its tags.
//
// It panics on a nil or non-comparable key. This is intentional and mirrors
// the standard library's context.WithValue contract: a nil or non-comparable
// key is a programmer error that should fail fast rather than be silently
// dropped, since a no-op would lose the value and mask the bug. Always pass a
// comparable, non-nil key.
func WithValue(parent context.Context, key, val any) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	tags := make(map[any]any)
	tags[key] = val
	return &Context{Context: parent, tags: tags, tagsLock: &sync.Mutex{}}
}

// WithLocalValue sets key/val on the existing ctx's tags and returns ctx.
//
// Like WithValue, it panics on a nil or non-comparable key to mirror the
// standard library's context.WithValue contract (see WithValue for rationale).
func WithLocalValue(ctx *Context, key, val any) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	ctx.tagsLock.Lock()
	if ctx.tags == nil {
		ctx.tags = make(map[any]any)
	}
	ctx.tags[key] = val
	ctx.tagsLock.Unlock()
	return ctx
}

// IsShareContext checks whether a context is share.Context.
func IsShareContext(ctx context.Context) bool {
	ok := ctx.Value(isShareContext)
	return ok != nil
}
