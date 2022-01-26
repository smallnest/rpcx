package client

import (
	"errors"
	"math/rand"
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

func TestCircuitBreakerRace(t *testing.T) {
	cb := NewConsecCircuitBreaker(2, 50*time.Millisecond)
	routines := 100
	loop := 100000

	fn := func() error {
		if rand.Intn(2) == 1 {
			return nil
		}
		return errors.New("test error")
	}

	for r := 0; r < routines; r++ {
		go func() {
			for i := 0; i < loop; i++ {
				cb.Call(fn, 100*time.Millisecond)
			}
		}()
	}
}
