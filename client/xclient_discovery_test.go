package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/smallnest/rpcx/protocol"
)

// stubClient is a no-op RPCClient used as cached entries in discovery tests.
type stubClient struct{}

func (m *stubClient) Connect(network, address string) error { return nil }
func (m *stubClient) Go(ctx context.Context, sp, sm string, args, reply any, done chan *Call) *Call {
	return nil
}
func (m *stubClient) Call(ctx context.Context, sp, sm string, args, reply any) error {
	return nil
}
func (m *stubClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	return nil, nil, nil
}
func (m *stubClient) Close() error                                          { return nil }
func (m *stubClient) RemoteAddr() string                                    { return "" }
func (m *stubClient) RegisterServerMessageChan(ch chan<- *protocol.Message) {}
func (m *stubClient) UnregisterServerMessageChan()                          {}
func (m *stubClient) IsClosing() bool                                       { return false }
func (m *stubClient) IsShutdown() bool                                      { return false }
func (m *stubClient) GetConn() net.Conn                                     { return nil }

// TestGetCachedClientDialDoesNotBlockOtherKeys asserts that while a dial for
// key A is in flight (blocked in Connect), a getCachedClient for an
// already-cached key B returns promptly. Before the fix, c.mu was held across
// the blocking Connect, serializing all callers of the xClient.
func TestGetCachedClientDialDoesNotBlockOtherKeys(t *testing.T) {
	c := &xClient{
		cachedClient: make(map[string]RPCClient),
		Plugins:      &pluginContainer{},
	}

	const slowKey = "tcp@slow:1234"
	const fastKey = "tcp@fast:5678"

	// Pre-cache a ready client for fastKey so getCachedClient(fastKey) hits the
	// fast (cache-only) path.
	fast := &stubClient{}
	c.cachedClient[fastKey] = fast

	slow := &stubClient{}
	dialBlock := make(chan struct{}) // GenerateClient(slowKey) blocks until closed
	dialing := make(chan struct{})   // closed when the slow dial starts

	// Route the dial for slowKey through a builder whose GenerateClient blocks,
	// simulating a slow/hanging Connect.
	RegisterCacheClientBuilder("tcp", &mockBuilder{
		cache: c.cachedClient,
		gen: func(k string) (RPCClient, error) {
			close(dialing)
			<-dialBlock
			return slow, nil
		},
	})
	t.Cleanup(func() {
		cacheClientBuildersMutex.Lock()
		delete(cacheClientBuilders, "tcp")
		cacheClientBuildersMutex.Unlock()
	})

	// Start a slow dial for slowKey in the background.
	dialDone := make(chan struct{})
	go func() {
		_, _ = c.getCachedClient(slowKey, "p", "m", nil)
		close(dialDone)
	}()

	// Wait until the slow dial is actually in GenerateClient (dialing).
	select {
	case <-dialing:
	case <-time.After(2 * time.Second):
		t.Fatal("slow dial never started")
	}

	// Now a lookup for the already-cached fastKey must return promptly.
	got := make(chan RPCClient, 1)
	go func() {
		cl, err := c.getCachedClient(fastKey, "p", "m", nil)
		if err != nil {
			t.Errorf("getCachedClient(fastKey): %v", err)
		}
		got <- cl
	}()

	select {
	case cl := <-got:
		if cl != fast {
			t.Fatalf("expected cached fast client, got %v", cl)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("getCachedClient(fastKey) blocked behind slow dial — c.mu held across Connect")
	}

	// Release the slow dial and let it finish.
	close(dialBlock)
	<-dialDone
}

// mockBuilder is a CacheClientBuilder backed by the same map as the xClient
// under test. Its cache ops run under the xClient's c.mu (getCachedClient holds
// it around FindCachedClient/SetCachedClient), while GenerateClient runs
// lock-free — exactly the property under test.
type mockBuilder struct {
	cache map[string]RPCClient
	gen   func(k string) (RPCClient, error)
}

func (b *mockBuilder) SetCachedClient(client RPCClient, k, sp, sm string) { b.cache[k] = client }
func (b *mockBuilder) FindCachedClient(k, sp, sm string) RPCClient        { return b.cache[k] }
func (b *mockBuilder) DeleteCachedClient(client RPCClient, k, sp, sm string) {
	delete(b.cache, k)
}
func (b *mockBuilder) GenerateClient(k, sp, sm string) (RPCClient, error) { return b.gen(k) }
