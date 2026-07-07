package server

import (
	"crypto/tls"
	"time"

	"github.com/alitto/pond"
	"github.com/smallnest/rpcx/protocol"
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

// WithMaxMessageLength caps the wire (compressed) length of an incoming
// message. It sets protocol.MaxMessageLength. A value <= 0 means no limit.
func WithMaxMessageLength(maxLen int) OptionFn {
	return func(s *Server) {
		protocol.MaxMessageLength = maxLen
	}
}

// WithMaxDecompressedLength caps the size (in bytes) of a message payload
// after decompression. It sets protocol.MaxDecompressedLength, guarding
// against decompression-bomb attacks where a small compressed payload expands
// to gigabytes of memory during decode. A value <= 0 means no limit.
func WithMaxDecompressedLength(maxLen int64) OptionFn {
	return func(s *Server) {
		protocol.MaxDecompressedLength = maxLen
	}
}
