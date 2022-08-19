package client

import (
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
