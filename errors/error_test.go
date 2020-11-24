package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMultiError(t *testing.T) {
	var errorSet []error
	errorSet = append(errorSet, errors.New("invalid"))
	errorSet = append(errorSet, errors.New("fatal"))

	multiError := NewMultiError(errorSet)
	assert.Equal(t, fmt.Sprintf("%v", errorSet), multiError.Error(), "Test NewMultiError()")
}

func TestMultiError_Append(t *testing.T) {
	multiErrors := MultiError{}
	multiErrors.Errors = append(multiErrors.Errors, errors.New("invalid"))
	multiErrors.Errors = append(multiErrors.Errors, errors.New("fatal"))

	assert.Equal(t, 2, len(multiErrors.Errors), "Test Append()")
}

func TestMultiError_Error(t *testing.T) {
	multiErrors := MultiError{}
	multiErrors.Errors = append(multiErrors.Errors, errors.New("invalid"))
	multiErrors.Errors = append(multiErrors.Errors, errors.New("fatal"))

	assert.Equal(t, "[invalid fatal]",multiErrors.Error(), "Test Error()")
}