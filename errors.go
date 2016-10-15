package rpcx

import (
	"fmt"
	"runtime"
)

var (
	// ErrPluginAlreadyExists returns an error with message: 'Cannot activate the same plugin again, plugin '+plugin name' is already exists'
	ErrPluginAlreadyExists = NewRPCError("Cannot use the same plugin again, '%s' is already exists")
	// ErrPluginActivate returns an error with message: 'While trying to activate plugin '+plugin name'. Trace: +specific error'
	ErrPluginActivate = NewRPCError("While trying to activate plugin '%s'. Trace: %s")
	// ErrPluginRemoveNoPlugins returns an error with message: 'No plugins are registed yet, you cannot remove a plugin from an empty list!'
	ErrPluginRemoveNoPlugins = NewRPCError("No plugins are registed yet, you cannot remove a plugin from an empty list!")
	// ErrPluginRemoveEmptyName returns an error with message: 'Plugin with an empty name cannot be removed'
	ErrPluginRemoveEmptyName = NewRPCError("Plugin with an empty name cannot be removed")
	// ErrPluginRemoveNotFound returns an error with message: 'Cannot remove a plugin which doesn't exists'
	ErrPluginRemoveNotFound = NewRPCError("Cannot remove a plugin which doesn't exists")
	// Context other

)

// RPCError holds the error
type RPCError struct {
	message string
}

// Error returns the message of the actual error
func (e *RPCError) Error() string {
	return e.message
}

// Format returns a formatted new error based on the arguments
func (e *RPCError) Format(args ...interface{}) error {
	return fmt.Errorf(e.message, args)
}

// With does the same thing as Format but it receives an error type which if it's nil it returns a nil error
func (e *RPCError) With(err error) error {
	if err == nil {
		return nil
	}

	return e.Format(err.Error())
}

// Return returns the actual error as it is
func (e *RPCError) Return() error {
	return fmt.Errorf(e.message)
}

// Panic output the message and after panics
func (e *RPCError) Panic() {
	if e == nil {
		return
	}
	_, fn, line, _ := runtime.Caller(1)
	errMsg := e.message
	errMsg = "\nCaller was: " + fmt.Sprintf("%s:%d", fn, line)
	panic(errMsg)
}

// Panicf output the formatted message and after panics
func (e *RPCError) Panicf(args ...interface{}) {
	if e == nil {
		return
	}
	_, fn, line, _ := runtime.Caller(1)
	errMsg := e.Format(args...).Error()
	errMsg = "\nCaller was: " + fmt.Sprintf("%s:%d", fn, line)
	panic(errMsg)
}

// NewRPCError creates and returns an Error with a message
func NewRPCError(errMsg string) *RPCError {
	return &RPCError{message: "\n" + "Error: " + errMsg}
}

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
