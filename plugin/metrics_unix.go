// +build !windows,!nacl,!plan9

package plugin

import (
	"log/syslog"
	"time"

	"github.com/rcrowley/go-metrics"
)

// Syslog reports metrics into syslog.
//
// 	w, _ := syslog.Dial("unixgram", "/dev/log", syslog.LOG_INFO, "metrics")
//	p.Syslog(60e9, w)
//
func (p *MetricsPlugin) Syslog(freq time.Duration, w *syslog.Writer) {
	go metrics.Syslog(p.Registry, freq, w)
}
