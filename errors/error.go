package errors

import "fmt"

// MultiError holds multiple errors
type MultiError struct {
	Errors []error
}

// Error returns the message of the actual error
func (e *MultiError) Error() string {
	return fmt.Sprintf("%v", e.Errors)
}

// NewMultiError creates and returns an Error with error splice
func NewMultiError(errors []error) *MultiError {
	return &MultiError{Errors: errors}
}
