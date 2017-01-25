// +build !windows,!nacl,!plan9

package clientselector

import (
	"os"
	"strings"
	"testing"

	"github.com/smallnest/rpcx/log"
)

func TestPing(t *testing.T) {
	if os.Getenv("travis") != "" {
		log.Infof("don't test this case in travis-ci.org")
		return
	}
	hosts := []string{"www.163.com", "www.baidu.com", "www.qq.com", "www.taobao.com"}

	for _, h := range hosts {
		rtt, err := Ping(h)

		if err != nil {
			if strings.Contains(err.Error(), "socket: permission denied") {
				t.Log("The Integration server doesn't allow socket operation")
			} else {
				t.Errorf("ping %s error: %s \n", h, err.Error())
			}
		} else {
			t.Logf("ping %s: %d \n", h, rtt)
		}
	}

	//Output
	// ping_utils_test.go:14: ping www.163.com: 272
	// ping_utils_test.go:14: ping www.baidu.com: 107
	// ping_utils_test.go:14: ping www.qq.com: 324
	// ping_utils_test.go:14: ping www.taobao.com: 306
}
