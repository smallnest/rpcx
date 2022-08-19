package server

import "time"

// WithTCPKeepAlivePeriod sets tcp keepalive period.
func WithTCPKeepAlivePeriod(period time.Duration) OptionFn {
	return func(s *Server) {
		s.options["TCPKeepAlivePeriod"] = period
	}
}
