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

// NewMultiError creates and returns an Error with error splice
func NewMultiError(errors []error) *MultiError {
	return &MultiError{Errors: errors}
}
