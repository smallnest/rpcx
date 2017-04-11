package core

import (
	"context"
)

type headerKey struct{}

// NewContext creates a new context with header attached.
func NewContext(ctx context.Context, header Header) context.Context {
	return context.WithValue(ctx, headerKey{}, header)
}

// FromContext returns the Header in ctx if it exists.
// The returned md should be immutable, writing to it may cause races.
// Modification should be made to the copies of the returned header.
func FromContext(ctx context.Context) (header Header, ok bool) {
	header, ok = ctx.Value(headerKey{}).(Header)
	return
}

type mapKey struct{}

// NewMapContext creates a new context with map[string]interface{} attached.
func NewMapContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, mapKey{}, make(map[string]interface{}))
}

// FromMapContext returns the map[string]interface{} in ctx if it exists.
// The returned md should be immutable, writing to it may cause races.
// Modification should be made to the copies of the returned map.
func FromMapContext(ctx context.Context) (m map[string]interface{}, ok bool) {
	m, ok = ctx.Value(mapKey{}).(map[string]interface{})
	return
}
