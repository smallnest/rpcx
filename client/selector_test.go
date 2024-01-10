package client

import (
	"context"
	"testing"
)

func Test_consistentHashSelector_Select(t *testing.T) {
	servers := map[string]string{
		"tcp@192.168.1.16:9392": "",
		"tcp@192.168.1.16:9393": "",
	}
	s := newConsistentHashSelector(servers).(*consistentHashSelector)

	key := uint64(9280147620691907957)
	selected, _ := s.h.Get(key).(string)

	for i := 0; i < 10000; i++ {
		selected2, _ := s.h.Get(key).(string)
		if selected != selected2 {
			t.Errorf("expected %s but got %s", selected, selected2)
		}
	}
}

func Test_consistentHashSelector_UpdateServer(t *testing.T) {
	servers := map[string]string{
		"tcp@192.168.1.16:9392": "",
		"tcp@192.168.1.16:9393": "",
	}
	s := newConsistentHashSelector(servers).(*consistentHashSelector)
	if len(s.h.All()) != len(servers) {
		t.Errorf("NewSelector: expected %d server but got %d", len(servers), len(s.h.All()))
	}
	s.UpdateServer(servers)
	if len(s.h.All()) != len(servers) {
		t.Errorf("UpdateServer: expected %d server but got %d", len(servers), len(s.h.All()))
	}
}

func TestWeightedRoundRobinSelector_Select(t *testing.T) {
	// a b a c a b a a b a c a b a
	sers := []string{"ServerA", "ServerB", "ServerA", "ServerC", "ServerA", "ServerB", "ServerA",
		"ServerA", "ServerB", "ServerA", "ServerC", "ServerA", "ServerB", "ServerA"}
	servers := make(map[string]string)
	servers["ServerA"] = "weight=4"
	servers["ServerB"] = "weight=2"
	servers["ServerC"] = "weight=1"
	ctx := context.Background()
	weightSelector := newWeightedRoundRobinSelector(servers).(*weightedRoundRobinSelector)

	for i := 0; i < 14; i++ {
		s := weightSelector.Select(ctx, "", "", nil)
		if s != sers[i] {
			t.Errorf("expected %s but got %s", sers[i], s)
		}
	}
}

func BenchmarkWeightedRoundRobinSelector_Select(b *testing.B) {
	servers := make(map[string]string)
	servers["ServerA"] = "weight=4"
	servers["ServerB"] = "weight=2"
	servers["ServerC"] = "weight=1"
	ctx := context.Background()
	weightSelector := newWeightedRoundRobinSelector(servers).(*weightedRoundRobinSelector)

	for i := 0; i < b.N; i++ {
		weightSelector.Select(ctx, "", "", nil)
	}
}
