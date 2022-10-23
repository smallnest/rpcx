package server

import (
	"crypto/tls"
	"time"

	"github.com/alitto/pond"
)

// OptionFn configures options of server.
type OptionFn func(*Server)

// // WithOptions sets multiple options.
// func WithOptions(ops map[string]interface{}) OptionFn {
// 	return func(s *Server) {
// 		for k, v := range ops {
// 			s.options[k] = v
// 		}
// 	}
// }

// WithTLSConfig sets tls.Config.
func WithTLSConfig(cfg *tls.Config) OptionFn {
	return func(s *Server) {
		s.tlsConfig = cfg
	}
}

// WithReadTimeout sets readTimeout.
func WithReadTimeout(readTimeout time.Duration) OptionFn {
	return func(s *Server) {
		s.readTimeout = readTimeout
	}
}

// WithWriteTimeout sets writeTimeout.
func WithWriteTimeout(writeTimeout time.Duration) OptionFn {
	return func(s *Server) {
		s.writeTimeout = writeTimeout
	}
}

// WithPool sets goroutine pool.
func WithPool(maxWorkers, maxCapacity int, options ...pond.Option) OptionFn {
	return func(s *Server) {
		s.pool = pond.New(maxWorkers, maxCapacity, options...)
	}
}

// WithCustomPool uses a custom goroutine pool.
func WithCustomPool(pool WorkerPool) OptionFn {
	return func(s *Server) {
		s.pool = pool
	}
}

// WithAsyncWrite sets AsyncWrite to true.
func WithAsyncWrite() OptionFn {
	return func(s *Server) {
		s.AsyncWrite = true
	}
}
