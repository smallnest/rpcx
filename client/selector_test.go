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
	calc := make(map[string]int)
	servers := make(map[string]string)
	servers["ServerA"] = "weight=4"
	servers["ServerB"] = "weight=2"
	servers["ServerC"] = "weight=1"
	weightSelector := newWeightedRoundRobinSelector(servers).(*weightedRoundRobinSelector)
	ctx := context.Background()
	for i := 0; i < 7; i++ {
		s := weightSelector.Select(ctx, "", "", nil)
		if _, ok := calc[s]; ok {
			calc[s]++
		} else {
			calc[s] = 1
		}
	}
	if calc["ServerA"] != 4 {
		t.Errorf("expected %d but got %d", 4, calc["ServerA"])
	}
	if calc["ServerB"] != 2 {
		t.Errorf("expected %d but got %d", 2, calc["ServerB"])
	}
	if calc["ServerC"] != 1 {
		t.Errorf("expected %d but got %d", 1, calc["ServerC"])
	}
}
func TestWeightedRoundRobinSelector_UpdateServer(t *testing.T) {
	calc := make(map[string]int)
	servers := make(map[string]string)
	servers["ServerA"] = "weight=4"
	servers["ServerB"] = "weight=2"
	servers["ServerC"] = "weight=1"
	weightSelector := newWeightedRoundRobinSelector(servers).(*weightedRoundRobinSelector)
	ctx := context.Background()
	servers["ServerA"] = "weight=5"
	weightSelector.UpdateServer(servers)
	for i := 0; i < 8; i++ {
		s := weightSelector.Select(ctx, "", "", nil)
		if _, ok := calc[s]; ok {
			calc[s]++
		} else {
			calc[s] = 1
		}
	}
	if calc["ServerA"] != 5 {
		t.Errorf("expected %d but got %d", 4, calc["ServerA"])
	}
	if calc["ServerB"] != 2 {
		t.Errorf("expected %d but got %d", 2, calc["ServerB"])
	}
	if calc["ServerC"] != 1 {
		t.Errorf("expected %d but got %d", 1, calc["ServerC"])
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

//
//func TestWeightedICMPSelector(t *testing.T) {
//	calc := make(map[string]int)
//	servers := make(map[string]string)
//	servers["@localhost:3333"] = ""
//	servers["@www.baidu.com:3334"] = ""
//	servers["@xxxx.xxxx:333"] = ""
//	s := newWeightedICMPSelector(servers)
//	ctx := context.Background()
//	for i := 0; i < 10; i++ {
//		host := s.Select(ctx, "", "", nil)
//		if _, ok := calc[host]; ok {
//			calc[host]++
//		} else {
//			calc[host] = 0
//		}
//	}
//	if len(calc) != 2 {
//		t.Errorf("expected %d but got %d", 2, len(servers))
//	}
//}
//func TestWeightedICMPSelector_UpdateServer(t *testing.T) {
//	calc := make(map[string]int)
//	servers := make(map[string]string)
//	servers["@localhost:3333"] = ""
//	servers["@www.baidu.com:3334"] = ""
//	servers["@xxxx.xxxx:333"] = ""
//	s := newWeightedICMPSelector(servers)
//	ctx := context.Background()
//	servers["@www.sina.com:3333"] = ""
//	s.UpdateServer(servers)
//	for i := 0; i < 10; i++ {
//		host := s.Select(ctx, "", "", nil)
//		if _, ok := calc[host]; ok {
//			calc[host]++
//		} else {
//			calc[host] = 0
//		}
//	}
//	if len(calc) != 3 {
//		t.Errorf("expected %d but got %d", 3, len(servers))
//	}
//}
