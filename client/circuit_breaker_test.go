package client

import (
	"errors"
	"testing"
	"time"
)

func TestConsecCircuitBreaker(t *testing.T) {
	count := -1
	fn := func() error {
		count++
		if count >= 5 && count < 10 {
			return nil
		}

		return errors.New("test error")
	}

	cb := NewConsecCircuitBreaker(5, 100*time.Millisecond)

	for i := 0; i < 25; i++ {
		err := cb.Call(fn, 200*time.Millisecond)
		switch {
		case i < 5:
			if err.Error() != "test error" {
				t.Fatalf("expect %v, got %v", "test error", err)
			}
		case i >= 5 && i < 10:
			if err != ErrBreakerOpen {
				t.Fatalf("expect %v, got %v", ErrBreakerOpen, err)
			}
		case i >= 10 && i < 15:
			if err != nil {
				t.Fatalf("expect success, got %v", err)
			}
		case i >= 15 && i < 20:
			if err.Error() != "test error" {
				t.Fatalf("expect %v, got %v", "test error", err)
			}
		case i >= 20 && i < 25:
			if err != ErrBreakerOpen {
				t.Fatalf("expect %v, got %v", ErrBreakerOpen, err)
			}
		}

		if i == 9 { // expired
			time.Sleep(150 * time.Millisecond)
		}

	}

}
