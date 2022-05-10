package serverplugin

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimitingPlugin(t *testing.T) {
	r := NewRateLimitingPlugin(1*time.Second, 1)
	assert.Equal(t, 1*time.Second, r.FillInterval)
	assert.Equal(t, int64(1), r.Capacity)
}

func TestRateLimiting(t *testing.T) {
	var conn net.Conn
	r := NewRateLimitingPlugin(1*time.Second, 1)
	_, b := r.HandleConnAccept(conn)
	assert.Equal(t, true, b)

	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, false, b)

	time.Sleep(1 * time.Second)

	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, true, b)

	r = NewRateLimitingPlugin(1*time.Second, 3)
	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, true, b)
	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, true, b)
	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, true, b)
	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, false, b)

	time.Sleep(1 * time.Second)

	_, b = r.HandleConnAccept(conn)
	assert.Equal(t, true, b)

}
