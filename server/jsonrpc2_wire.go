// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// this file contains the go forms of the wire specification
// see http://www.jsonrpc.org/specification for details

const (
	// CodeUnknownJSONRPCError should be used for all non coded errors.
	CodeUnknownJSONRPCError = -32001
	// CodeParseJSONRPCError is used when invalid JSON was received by the server.
	CodeParseJSONRPCError = -32700
	//CodeInvalidjsonrpcRequest is used when the JSON sent is not a valid jsonrpcRequest object.
	CodeInvalidjsonrpcRequest = -32600
	// CodeMethodNotFound should be returned by the handler when the method does
	// not exist / is not available.
	CodeMethodNotFound = -32601
	// CodeInvalidParams should be returned by the handler when method
	// parameter(s) were invalid.
	CodeInvalidParams = -32602
	// CodeInternalJSONRPCError is not currently returned but defined for completeness.
	CodeInternalJSONRPCError = -32603
)

// jsonrpcRequest is sent to a server to represent a Call or Notify operation.
type jsonrpcRequest struct {
	// VersionTag is always encoded as the string "2.0"
	VersionTag VersionTag `json:"jsonrpc"`
	// Method is a string containing the method name to invoke.
	Method string `json:"method"`
	// Params is either a struct or an array with the parameters of the method.
	Params *json.RawMessage `json:"params,omitempty"`
	// The id of this request, used to tie the jsonrpcRespone back to the request.
	// Will be either a string or a number. If not set, the jsonrpcRequest is a notify,
	// and no response is possible.
	ID *ID `json:"id,omitempty"`
}

// jsonrpcRespone is a reply to a jsonrpcRequest.
// It will always have the ID field set to tie it back to a request, and will
// have either the Result or JSONRPCError fields set depending on whether it is a
// success or failure response.
type jsonrpcRespone struct {
	// VersionTag is always encoded as the string "2.0"
	VersionTag VersionTag `json:"jsonrpc"`
	// Result is the response value, and is required on success.
	Result *json.RawMessage `json:"result,omitempty"`
	// Error is a structured error response if the call fails.
	Error *JSONRPCError `json:"error,omitempty"`
	// ID must be set and is the identifier of the jsonrpcRequest this is a response to.
	ID *ID `json:"id,omitempty"`
}

// JSONRPCError represents a structured error in a jsonrpcRespone.
type JSONRPCError struct {
	// Code is an error code indicating the type of failure.
	Code int64 `json:"code"`
	// Message is a short description of the error.
	Message string `json:"message"`
	// Data is optional structured data containing additional information about the error.
	Data *json.RawMessage `json:"data"`
}

// VersionTag is a special 0 sized struct that encodes as the jsonrpc version
// tag.
// It will fail during decode if it is not the correct version tag in the
// stream.
type VersionTag struct{}

// ID is a jsonrpcRequest identifier.
// Only one of either the Name or Number members will be set, using the
// number form if the Name is the empty string.
type ID struct {
	Name   string
	Number int64
}

// IsNotify returns true if this request is a notification.
func (r *jsonrpcRequest) IsNotify() bool {
	return r.ID == nil
}

func (err *JSONRPCError) JSONRPCError() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func (VersionTag) MarshalJSON() ([]byte, error) {
	return json.Marshal("2.0")
}

func (VersionTag) UnmarshalJSON(data []byte) error {
	version := ""
	if err := json.Unmarshal(data, &version); err != nil {
		return err
	}
	if version != "2.0" {
		return fmt.Errorf("invalid RPC version %v", version)
	}
	return nil
}

// String returns a string representation of the ID.
// The representation is non ambiguous, string forms are quoted, number forms
// are preceded by a #
func (id *ID) String() string {
	if id == nil {
		return ""
	}
	if id.Name != "" {
		return strconv.Quote(id.Name)
	}
	return "#" + strconv.FormatInt(id.Number, 10)
}

func (id *ID) MarshalJSON() ([]byte, error) {
	if id.Name != "" {
		return json.Marshal(id.Name)
	}
	return json.Marshal(id.Number)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	*id = ID{}
	if err := json.Unmarshal(data, &id.Number); err == nil {
		return nil
	}
	return json.Unmarshal(data, &id.Name)
}
