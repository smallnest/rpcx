package errors

import (
	"fmt"
	"sync"
)

// MultiError holds multiple errors
type MultiError struct {
	Errors []error
	mu     sync.Mutex
}

// Error returns the message of the actual error
func (e *MultiError) Error() string {
	return fmt.Sprintf("%v", e.Errors)
}

func (e *MultiError) Append(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Errors = append(e.Errors, err)
}

func (e *MultiError) ErrorOrNil() error {
	if e == nil || len(e.Errors) == 0 {
		return nil
	}
	return e
}

// NewMultiError creates and returns an Error with error splice
func NewMultiError(errors []error) *MultiError {
	return &MultiError{Errors: errors}
}
