package server

import (
	"reflect"
	"sync"
)

var UsePool bool

// Reset defines Reset method for pooled object.
type Reset interface {
	Reset()
}

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

type typePools struct {
	lock  sync.Mutex
	pools map[reflect.Type]*sync.Pool
	New   func(t reflect.Type) interface{}
}

func (p *typePools) Init(t reflect.Type) {
	tp := &sync.Pool{}
	tp.New = func() interface{} {
		return p.New(t)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.pools[t] = tp
}

func (p *typePools) Put(t reflect.Type, x interface{}) {
	if !UsePool {
		return
	}
	if o, ok := x.(Reset); ok {
		o.Reset()
	}
	p.pools[t].Put(x)
}

func (p *typePools) Get(t reflect.Type) interface{} {
	if !UsePool {
		return p.New(t)
	}
	return p.pools[t].Get()
}
