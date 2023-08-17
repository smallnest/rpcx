package serverplugin

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
)

var _ server.PostReadRequestPlugin = (*RedisRateLimitingPlugin)(nil)

// RedisRateLimitingPlugin can limit requests per unit time
type RedisRateLimitingPlugin struct {
	addrs   []string
	limiter redis_rate.Limiter
	limit   redis_rate.Limit
}

// NewRedisRateLimitingPlugin creates a new RateLimitingPlugin
func NewRedisRateLimitingPlugin(addrs []string, rate int, burst int, period time.Duration) *RedisRateLimitingPlugin {
	limit := redis_rate.Limit{
		Rate:   rate,
		Burst:  burst,
		Period: period,
	}
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})

	limiter := redis_rate.NewLimiter(rdb)

	return &RedisRateLimitingPlugin{
		addrs:   addrs,
		limiter: *limiter,
		limit:   limit,
	}
}

// PostReadRequest can limit request processing.
func (plugin *RedisRateLimitingPlugin) PostReadRequest(ctx context.Context, r *protocol.Message, e error) error {
	res, err := plugin.limiter.Allow(ctx, r.ServicePath+"/"+r.ServiceMethod, plugin.limit)
	if err != nil {
		return err
	}

	if res.Allowed > 0 {
		return nil
	}
	return server.ErrReqReachLimit
}
