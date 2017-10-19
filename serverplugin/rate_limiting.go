package serverplugin

import (
	"net"
	"time"

	"github.com/juju/ratelimit"
)

// RateLimitingPlugin can limit connecting per unit time
type RateLimitingPlugin struct {
	FillInterval time.Duration
	Capacity     int64
	bucket       *ratelimit.Bucket
}

// NewRateLimitingPlugin creates a new RateLimitingPlugin
func NewRateLimitingPlugin(fillInterval time.Duration, capacity int64) *RateLimitingPlugin {
	tb := ratelimit.NewBucket(fillInterval, capacity)

	return &RateLimitingPlugin{
		FillInterval: fillInterval,
		Capacity:     capacity,
		bucket:       tb}
}

// HandleConnAccept can limit connecting rate
func (plugin *RateLimitingPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	return conn, plugin.bucket.TakeAvailable(1) > 0
}
