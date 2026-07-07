package client

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
)

// Service discovery, server filtering, and cached-client management
// for xClient. Extracted from xclient.go.

// watch changes of service and update cached clients.
func (c *xClient) watch(ch chan []*KVPair) {
	for pairs := range ch {
		servers := make(map[string]string, len(pairs))
		for _, p := range pairs {
			servers[p.Key] = p.Value
		}
		c.mu.Lock()
		filterByStateAndGroup(c.option.Group, servers)
		c.servers = servers

		if c.selector != nil {
			c.selector.UpdateServer(servers)
		}

		c.mu.Unlock()
	}
}

func filterByStateAndGroup(group string, servers map[string]string) {
	for k, v := range servers {
		if values, err := url.ParseQuery(v); err == nil {
			if state := values.Get("state"); state == "inactive" {
				delete(servers, k)
			}
			// Membership test: does this server's (typically tiny) groups slice
			// contain the single target group? A linear scan is intentional here
			// — building a Set per server would add map-allocation overhead for a
			// single lookup against a 1-3 element slice.
			groups := values["group"]
			if group != "" {
				found := slices.Contains(groups, group)
				if !found {
					delete(servers, k)
				}
			}
		}
	}
}

// selects a client from candidates base on c.selectMode
func (c *xClient) selectClient(ctx context.Context, servicePath, serviceMethod string, args any) (string, RPCClient, error) {
	c.mu.Lock()

	if c.option.Sticky && c.stickyRPCClient != nil {
		if c.stickyRPCClient.IsClosing() || c.stickyRPCClient.IsShutdown() {
			c.stickyRPCClient = nil
		}
	}

	if c.option.Sticky && c.stickyRPCClient != nil {
		c.mu.Unlock()
		return c.stickyK, c.stickyRPCClient, nil
	}

	fn := c.selector.Select
	if c.Plugins != nil {
		fn = c.Plugins.DoWrapSelect(fn)
	}
	k := fn(ctx, servicePath, serviceMethod, args)
	c.mu.Unlock()

	if k == "" {
		return "", nil, ErrXClientNoServer
	}

	client, err := c.getCachedClient(k, servicePath, serviceMethod, args)

	if c.option.Sticky && client != nil {
		c.mu.Lock()
		safeCloseClient(c.stickyRPCClient)

		c.stickyK = k
		c.stickyRPCClient = client
		c.mu.Unlock()
	}

	return k, client, err
}

func safeCloseClient(client RPCClient) {
	if client == nil {
		return
	}

	defer func() {
		_ = recover()
	}()

	client.Close()
}

func (c *xClient) getCachedClient(k string, servicePath, serviceMethod string, _ any) (client RPCClient, err error) {
	var needCallPlugin bool
	defer func() {
		if needCallPlugin {
			_, err = c.Plugins.DoClientConnected(client.GetConn())
		}
	}()

	if c.isShutdown {
		return nil, errors.New("this xclient is closed")
	}

	// if this client is broken
	breaker, ok := c.breakers.Load(k)
	if ok && !breaker.(Breaker).Ready() {
		return nil, ErrBreakerOpen
	}

	// Fast path: only the cache map lookup is guarded by c.mu.
	c.mu.Lock()
	client = c.findCachedClient(k, servicePath, serviceMethod)
	if client != nil {
		if !client.IsClosing() && !client.IsShutdown() {
			c.mu.Unlock()
			return client, nil
		}
		c.deleteCachedClient(client, k, servicePath, serviceMethod)
		client = nil
	}
	c.mu.Unlock()

	// Slow path: dial outside c.mu so a slow or hanging Connect cannot block
	// other callers of this xClient. singleflight dedups concurrent dials to
	// the same key; created is set only by the goroutine that actually dials,
	// so the DoClientConnected plugin fires exactly once per new connection.
	var created bool
	generatedClient, err, _ := c.slGroup.Do(k, func() (any, error) {
		// Re-check the cache: another goroutine may have cached a client for k
		// between our fast-path unlock and entering singleflight.
		c.mu.Lock()
		if cached := c.findCachedClient(k, servicePath, serviceMethod); cached != nil &&
			!cached.IsClosing() && !cached.IsShutdown() {
			c.mu.Unlock()
			return cached, nil
		}
		c.mu.Unlock()

		newClient, gerr := c.generateClient(k, servicePath, serviceMethod)
		if gerr != nil {
			return nil, gerr
		}
		newClient.RegisterServerMessageChan(c.serverMessageChan)

		c.mu.Lock()
		c.setCachedClient(newClient, k, servicePath, serviceMethod)
		c.mu.Unlock()

		created = true
		return newClient, nil
	})

	// forget k so a later dial for the same key is not deduped against this one
	c.slGroup.Forget(k)

	if err != nil {
		return nil, err
	}

	client = generatedClient.(RPCClient)
	if created && c.Plugins != nil {
		needCallPlugin = true
	}

	return client, nil
}

func (c *xClient) setCachedClient(client RPCClient, k, servicePath, serviceMethod string) {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok {
		builder.SetCachedClient(client, k, servicePath, serviceMethod)
		return
	}

	c.cachedClient[k] = client
}

func (c *xClient) findCachedClient(k, servicePath, serviceMethod string) RPCClient {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok {
		return builder.FindCachedClient(k, servicePath, serviceMethod)
	}

	return c.cachedClient[k]
}

func (c *xClient) deleteCachedClient(client RPCClient, k, servicePath, serviceMethod string) {
	network, _ := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok && client != nil {
		builder.DeleteCachedClient(client, k, servicePath, serviceMethod)
		client.Close()
		return
	}

	delete(c.cachedClient, k)
	if client != nil {
		client.Close()
	}
}

func (c *xClient) removeClient(k, servicePath, serviceMethod string, client RPCClient) {
	c.mu.Lock()
	if c.option.Sticky {
		c.stickyK = ""
		c.stickyRPCClient = nil
	}

	cl := c.findCachedClient(k, servicePath, serviceMethod)
	if cl == client {
		c.deleteCachedClient(client, k, servicePath, serviceMethod)
	}
	c.mu.Unlock()

	if client != nil {
		client.UnregisterServerMessageChan()
		client.Close()
	}
}

func (c *xClient) generateClient(k, servicePath, serviceMethod string) (client RPCClient, err error) {
	network, addr := splitNetworkAndAddress(k)
	if builder, ok := getCacheClientBuilder(network); ok && builder != nil {
		return builder.GenerateClient(k, servicePath, serviceMethod)
	}

	client = &Client{
		option:  c.option,
		Plugins: c.Plugins,
	}

	var breaker any
	if c.option.GenBreaker != nil {
		breaker, _ = c.breakers.LoadOrStore(k, c.option.GenBreaker())
	}

	err = client.Connect(network, addr)
	if err != nil {
		if breaker != nil {
			breaker.(Breaker).Fail()
		}
		return nil, err
	}
	return client, err
}

func (c *xClient) getCachedClientWithoutLock(k, servicePath, serviceMethod string) (RPCClient, bool, error) {
	var needCallPlugin bool
	client := c.findCachedClient(k, servicePath, serviceMethod)
	if client != nil {
		if !client.IsClosing() && !client.IsShutdown() {
			return client, needCallPlugin, nil
		}
		c.deleteCachedClient(client, k, servicePath, serviceMethod)

		// double check
		client = c.findCachedClient(k, servicePath, serviceMethod)
	}

	if client == nil || client.IsShutdown() {
		generatedClient, err, _ := c.slGroup.Do(k, func() (any, error) {
			return c.generateClient(k, servicePath, serviceMethod)
		})

		if err != nil {
			c.slGroup.Forget(k)
			return nil, needCallPlugin, err
		}

		client = generatedClient.(RPCClient)
		if c.Plugins != nil {
			needCallPlugin = true
		}

		client.RegisterServerMessageChan(c.serverMessageChan)

		c.setCachedClient(client, k, servicePath, serviceMethod)
		c.slGroup.Forget(k)
	}

	return client, needCallPlugin, nil
}

func splitNetworkAndAddress(server string) (string, string) {
	ss := strings.SplitN(server, "@", 2)
	if len(ss) == 1 {
		return "tcp", server
	}

	return ss[0], ss[1]
}
