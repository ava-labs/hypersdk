// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/near/borsh-go"
)

type SimulatorRequest struct {
	// The API endpoint to call. (required)
	Endpoint Endpoint `json:"endpoint"`
	// The method to call on the endpoint.
	Method string `json:"method"`
	// The maximum number of units to consume for this request.
	MaxUnits uint64 `json:"maxUnits"`
	// The parameters to pass to the method.
	Params []Parameter `json:"params"`
}

type Endpoint string

const (
	/// Make a read-only call to a program function and return the result.
	ReadOnly Endpoint = "readonly"
	/// Create a transaction on-chain from a possible state changing program
	/// function call. A program's function can internally optionally call other
	/// functions including program to program.
	Execute Endpoint = "execute"
	/// Create a new program on-chain
	CreateProgram Endpoint = "createprogram"
)

func newResponse(id int) *Response {
	return &Response{
		ID: int(id),
		Error: "",
		Result: &Result{
			Response: []byte{},
		},
	}
}

type Response struct {
	// The index of the request that generated this response.
	ID int
	// The result of the request.
	Result *Result
	// The error message if available.
	Error string
}

func (r *Response) Print() error {
	// jsonBytes, err := json.Marshal(r)
	borshBytes, err := borsh.Serialize(r)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	fmt.Println(string(borshBytes))
	return nil
}

func (r *Response) setError(err error) {
	r.Error = err.Error()
}

func (r *Response) setBalance(balance uint64) {
	r.Result.Balance = balance
}

func (r *Response) setResponse(response []byte) {
	r.Result.Response = response
}

func (r *Response) setTimestamp(timestamp int64) {
	r.Result.Timestamp = uint64(timestamp)
}

type Result struct {
	// The tx id of the transaction that was created.
	ID string `json:"id,omitempty"`
	// The balance after the request has completed.
	Balance uint64 `json:"balance,omitempty"`
	// The response from the call.
	Response []byte `json:"response"`
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
	String      Type = "string"
	Path        Type = "path"
	Address     Type = "address"
	ID          Type = "id"
	TestContext Type = "testContext"
)

func unmarshalRequest(bytes []byte) (*SimulatorRequest, error) {
	var s SimulatorRequest
	if err := json.Unmarshal(bytes, &s); err != nil {
		return nil, err
	}

	return &s, nil
}
