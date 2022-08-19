package client

import (
	"errors"
	"sync/atomic"
	"time"
)

var (
	ErrBreakerOpen    = errors.New("breaker open")
	ErrBreakerTimeout = errors.New("breaker time out")
)

// ConsecCircuitBreaker is window sliding CircuitBreaker with failure threshold.
type ConsecCircuitBreaker struct {
	lastFailureTime int64

	failures         uint64
	failureThreshold uint64
	window           time.Duration
}

// NewConsecCircuitBreaker returns a new ConsecCircuitBreaker.
func NewConsecCircuitBreaker(failureThreshold uint64, window time.Duration) *ConsecCircuitBreaker {
	return &ConsecCircuitBreaker{
		failureThreshold: failureThreshold,
		window:           window,
	}
}

// Call Circuit function
func (cb *ConsecCircuitBreaker) Call(fn func() error, d time.Duration) error {
	var err error

	if !cb.ready() {
		return ErrBreakerOpen
	}

	if d == 0 {
		err = fn()
	} else {
		c := make(chan error, 1)
		go func() {
			c <- fn()
			close(c)
		}()

		t := time.NewTimer(d)
		select {
		case e := <-c:
			err = e
		case <-t.C:
			err = ErrBreakerTimeout
		}
		t.Stop()
	}

	if err == nil {
		cb.success()
	} else {
		cb.fail()
	}

	return err
}

func (cb *ConsecCircuitBreaker) ready() bool {
	lastFailureTime := time.Unix(0, atomic.LoadInt64(&cb.lastFailureTime))
	if time.Since(lastFailureTime) > cb.window {
		cb.reset()
		return true
	}

	failures := atomic.LoadUint64(&cb.failures)
	return failures < cb.failureThreshold
}

func (cb *ConsecCircuitBreaker) success() {
	cb.reset()
}

func (cb *ConsecCircuitBreaker) fail() {
	atomic.AddUint64(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())
}

func (cb *ConsecCircuitBreaker) Success() {
	cb.success()
}

func (cb *ConsecCircuitBreaker) Fail() {
	cb.fail()
}

func (cb *ConsecCircuitBreaker) Ready() bool {
	return cb.ready()
}

func (cb *ConsecCircuitBreaker) reset() {
	atomic.StoreUint64(&cb.failures, 0)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())
}
