package rpcx

import (
	"fmt"
	"runtime"
)

var (
	ErrPluginAlreadyExists   = NewRPCError("cannot use the same plugin again, '%s' is already exists")
	ErrPluginActivate        = NewRPCError("while trying to activate plugin '%s'. Trace: %s")
	ErrPluginRemoveNoPlugins = NewRPCError("no plugins are registed yet, you cannot remove a plugin from an empty list!")
	ErrPluginRemoveEmptyName = NewRPCError("plugin with an empty name cannot be removed")
	ErrPluginRemoveNotFound  = NewRPCError("cannot remove a plugin which doesn't exists")

	ErrWrongServiceMethod = NewRPCError("? is not allowed in service method name):%s")
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
	errMsg := e.message + "\nCaller was: " + fmt.Sprintf("%s:%d", fn, line)
	panic(errMsg)
}

// Panicf output the formatted message and after panics
func (e *RPCError) Panicf(args ...interface{}) {
	if e == nil {
		return
	}
	_, fn, line, _ := runtime.Caller(1)
	errMsg := e.Format(args...).Error()
	errMsg = errMsg + "\nCaller was: " + fmt.Sprintf("%s:%d", fn, line)
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
