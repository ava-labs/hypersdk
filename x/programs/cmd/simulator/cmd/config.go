// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"fmt"
)

const (
	ProgramCreate  = "program_create"
	ProgramExecute = "execute"
)

type Step struct {
	// The key of the caller used.
	CallerKey string `json:"callerKey"`
	// The API endpoint to call. (required)
	Endpoint Endpoint `json:"endpoint"`
	// The method to call on the endpoint.
	Method string `json:"method"`
	// The maximum number of units to consume for this step.
	MaxUnits uint64 `json:"maxUnits"`
	// The parameters to pass to the method.
	Params []Parameter `json:"params"`
}

type Endpoint string

const (
	/// Perform an operation against the key api.
	EndpointKey Endpoint = "key"
	/// Make a read-only call to a program function and return the result.
	EndpointReadOnly Endpoint = "readonly"
	/// Create a transaction on-chain from a possible state changing program
	/// function call. A program's function can internally optionally call other
	/// functions including program to program.
	EndpointExecute Endpoint = "execute"
)

func newResponse(id int) *Response {
	return &Response{
		ID: id,
		Result: &Result{
			Response: []byte{},
		},
	}
}

type Response struct {
	// The index of the step that generated this response.
	ID int `json:"id"`
	// The result of the step.
	Result *Result `json:"result,omitempty"`
	// The error message if available.
	Error string `json:"error,omitempty"`
}

func (r *Response) Print() error {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func (r *Response) setError(err error) {
	r.Error = err.Error()
}

func (r *Response) setTxID(id string) {
	r.Result.ID = id
}

func (r *Response) getTxID() (string, bool) {
	if r.Result.ID == "" {
		return "", false
	}
	return r.Result.ID, true
}

func (r *Response) setBalance(balance uint64) {
	r.Result.Balance = balance
}

func (r *Response) setResponse(response []byte) {
	r.Result.Response = response
}

func (r *Response) setMsg(msg string) {
	r.Result.Msg = msg
}

func (r *Response) setTimestamp(timestamp int64) {
	r.Result.Timestamp = uint64(timestamp)
}

type Result struct {
	// The tx id of the transaction that was created.
	ID string `json:"id,omitempty"`
	// The balance after the step has completed.
	Balance uint64 `json:"balance,omitempty"`
	// The response from the call.
	Response []byte `json:"response"`
	// An optional message.
	Msg string `json:"msg,omitempty"`
	// Timestamp of the response.
	Timestamp uint64 `json:"timestamp,omitempty"`
}

type Parameter struct {
	// The type of the parameter. (required)
	Type Type `json:"type"`
	// The value of the parameter. (required)
	Value []byte `json:"value"`
}

type Type string

const (
	String       Type = "string"
	Bool         Type = "bool"
	ID           Type = "id"
	KeyEd25519   Type = "ed25519"
	KeySecp256k1 Type = "secp256k1"
	Uint64       Type = "u64"
)

func unmarshalStep(bytes []byte) (*Step, error) {
	var s Step
	if err := json.Unmarshal(bytes, &s); err != nil {
		return nil, err
	}

	return &s, nil
}
