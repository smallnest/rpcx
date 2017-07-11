package clientselector

import (
	"net/rpc"
	"sync"
)

// CMap is a goutine-safe/thread-safe map
type CMap struct {
	data map[string]*rpc.Client
	sync.RWMutex
}

func NewCMap() *CMap {
	return &CMap{data: make(map[string]*rpc.Client)}
}

func (s *CMap) Set(key string, value *rpc.Client) {
	s.Lock()
	s.data[key] = value
	s.Unlock()
}
func (s *CMap) Get(key string) *rpc.Client {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}
func (s *CMap) Remove(key string) {
	s.RLock()
	defer s.RUnlock()
	delete(s.data, key)
}
