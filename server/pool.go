package server

import (
	"reflect"
	"sync"
)

var argsReplyPools = &typePools{
	pools: make(map[reflect.Type]*sync.Pool),
	New: func(t reflect.Type) interface{} {
		var argv reflect.Value

		if t.Kind() == reflect.Ptr { // reply must be ptr
			argv = reflect.New(t.Elem())
		} else {
			argv = reflect.New(t)
		}

		return argv.Interface()
	},
}

// this struct is not gororutine-safe.
type typePools struct {
	pools map[reflect.Type]*sync.Pool
	New   func(t reflect.Type) interface{}
}

func (p *typePools) Init(t reflect.Type) {
	tp := &sync.Pool{}
	tp.New = func() interface{} {
		return p.New(t)
	}
	p.pools[t] = tp
}

func (p *typePools) Put(t reflect.Type, x interface{}) {
	p.pools[t].Put(x)
}

func (p *typePools) Get(t reflect.Type) interface{} {
	return p.pools[t].Get()
}
