package client

import (
	"testing"
)

// testPlugin is a simple test plugin implementation
type testPlugin struct {
	name string
}

func TestXClientPool_SetPlugins(t *testing.T) {
	// Create a simple discovery
	pairs := []*KVPair{
		{Key: "tcp@127.0.0.1:8972", Value: ""},
	}
	discovery, err := NewMultipleServersDiscovery(pairs)
	if err != nil {
		t.Fatalf("failed to create discovery: %v", err)
	}
	defer discovery.Close()

	// Create a pool
	pool := NewXClientPool(3, "Arith", Failtry, RandomSelect, discovery, DefaultOption)
	defer pool.Close()

	// Create plugins
	plugins := NewPluginContainer()
	tp := &testPlugin{name: "test-plugin"}
	plugins.Add(tp)

	// Test SetPlugins
	pool.SetPlugins(plugins)

	// Verify plugins are set on pool
	if pool.GetPlugins() == nil {
		t.Error("plugins should not be nil after SetPlugins")
	}
	if pool.GetPlugins() != plugins {
		t.Error("pool plugins should be the same as the set plugins")
	}

	// Verify plugins are set on all xclients
	for i := 0; i < 3; i++ {
		xclient := pool.Get()
		if xclient.GetPlugins() == nil {
			t.Errorf("xclient %d plugins should not be nil", i)
		}
		if xclient.GetPlugins() != plugins {
			t.Errorf("xclient %d plugins should be the same as the set plugins", i)
		}
	}
}

func TestXClientPool_GetPlugins(t *testing.T) {
	// Create a simple discovery
	pairs := []*KVPair{
		{Key: "tcp@127.0.0.1:8972", Value: ""},
	}
	discovery, err := NewMultipleServersDiscovery(pairs)
	if err != nil {
		t.Fatalf("failed to create discovery: %v", err)
	}
	defer discovery.Close()

	// Create a pool
	pool := NewXClientPool(2, "Arith", Failtry, RandomSelect, discovery, DefaultOption)
	defer pool.Close()

	// Initially, plugins should be nil
	if pool.GetPlugins() != nil {
		t.Error("plugins should be nil initially")
	}

	// Create and set plugins
	plugins := NewPluginContainer()
	tp := &testPlugin{name: "test-plugin"}
	plugins.Add(tp)
	pool.SetPlugins(plugins)

	// Verify GetPlugins returns the correct plugins
	retrievedPlugins := pool.GetPlugins()
	if retrievedPlugins == nil {
		t.Error("plugins should not be nil after SetPlugins")
	}
	if retrievedPlugins != plugins {
		t.Error("GetPlugins should return the same plugins as SetPlugins")
	}

	// Verify plugins contain the test plugin
	allPlugins := retrievedPlugins.All()
	if len(allPlugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(allPlugins))
	}
	if p, ok := allPlugins[0].(*testPlugin); !ok || p.name != "test-plugin" {
		t.Error("plugin should be the test plugin")
	}
}

func TestXClientPool_SetPlugins_Concurrent(t *testing.T) {
	// Create a simple discovery
	pairs := []*KVPair{
		{Key: "tcp@127.0.0.1:8972", Value: ""},
	}
	discovery, err := NewMultipleServersDiscovery(pairs)
	if err != nil {
		t.Fatalf("failed to create discovery: %v", err)
	}
	defer discovery.Close()

	// Create a pool
	pool := NewXClientPool(5, "Arith", Failtry, RandomSelect, discovery, DefaultOption)
	defer pool.Close()

	// Test concurrent SetPlugins calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			plugins := NewPluginContainer()
			tp := &testPlugin{name: "test-plugin"}
			plugins.Add(tp)
			pool.SetPlugins(plugins)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify plugins are set
	if pool.GetPlugins() == nil {
		t.Error("plugins should not be nil after concurrent SetPlugins")
	}
}

