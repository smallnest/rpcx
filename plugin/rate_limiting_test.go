package plugin

import (
	"testing"
	"time"
)

func TestRateLimitingPlugin(t *testing.T) {
	p := NewRateLimitingPlugin(time.Second, 1000)
	time.Sleep(1 * time.Second)

	total := 0
	for i := 0; i < 2000; i++ {
		if p.HandleConnAccept(nil) {
			total++
		}
	}
	if total > 1100 {
		t.Errorf("rate limiting has not work. Handled: %d, Expected: about 1000", total)
	}
}
