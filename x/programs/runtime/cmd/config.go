// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/near/borsh-go"
	"gopkg.in/yaml.v2"
)

const (
	ProgramCreate  = "program_create"
	ProgramExecute = "execute"
)

type Step struct {
	// The key of the caller used.
	CallerKey string `yaml:"caller_key" json:"callerKey"`
	// The API endpoint to call. (required)
	Endpoint Endpoint `json:"endpoint" yaml:"endpoint"`
	// The method to call on the endpoint.
	Method string `json:"method" yaml:"method"`
	// The maximum number of units to consume for this step.
	MaxUnits uint64 `json:"maxUnits" yaml:"max_units"`
	// The parameters to pass to the method.
	Params []Parameter `json:"params" yaml:"params"`
	// Define required assertions against this step.
	Require *Require `json:"require,omitempty" yaml:"require,omitempty"`
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
		ID:     id,
		Result: &Result{},
	}
}

type Response struct {
	// The index of the step that generated this response.
	ID int `json:"id" yaml:"id"`
	// The result of the step.
	Result *Result `json:"result,omitempty" yaml:"result,omitempty"`
	// The error message if available.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
}

func (r *Response) Print() error {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %s", err)
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
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// The balance after the step has completed.
	Balance uint64 `json:"balance,omitempty" yaml:"balance,omitempty"`
	// The response from the call.
	Response []byte `json:"response" yaml:"response"`
	// An optional message.
	Msg string `json:"msg,omitempty" yaml:"msg,omitempty"`
	// Timestamp of the response.
	Timestamp uint64 `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

type Require struct {
	// Assertions against the result of the step.
	Result ResultAssertion `json,yaml:"result,omitempty"`
}

type ResultAssertion struct {
	// The operator to use for the assertion.
	Operator string `json,yaml:"operator"`
	// The value to compare against.
	Value string `json,yaml:"value"`
}

type Operator string

const (
	NumericGt Operator = ">"
	NumericLt Operator = "<"
	NumericGe Operator = ">="
	NumericLe Operator = "<="
	NumericEq Operator = "=="
	NumericNe Operator = "!="
	// TODO: Add string operators?
)

type Parameter struct {
	// The type of the parameter. (required)
	Type Type `json,yaml:"type"`
	// The value of the parameter. (required)
	Value interface{} `json,yaml:"value"`
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

// validateAssertion validates the assertion against the actual value.
func validateAssertion(bytes []byte, require *Require) (bool, error) {
	if require == nil {
		return true, nil
	}

	actual := int64(0)
	borsh.Deserialize(&actual, bytes)
	assertion := require.Result
	// convert the assertion value(string) to uint64
	value, err := strconv.ParseInt(assertion.Value, 10, 64)
	if err != nil {
		return false, err
	}

	switch Operator(assertion.Operator) {
	case NumericGt:
		if actual > value {
			return true, nil
		}
	case NumericLt:
		if actual < value {
			return true, nil
		}
	case NumericGe:
		if actual >= value {
			return true, nil
		}
	case NumericLe:
		if actual <= value {
			return true, nil
		}
	case NumericEq:
		if actual == value {
			return true, nil
		}
	case NumericNe:
		if actual != value {
			return true, nil
		}
	default:
		return false, fmt.Errorf("invalid assertion operator: %s", assertion.Operator)
	}

	return false, nil
}

func unmarshalStep(bytes []byte) (*Step, error) {
	var s Step
	switch {
	case isJSON(string(bytes)):
		if err := json.Unmarshal(bytes, &s); err != nil {
			return nil, err
		}
	case isYAML(string(bytes)):
		if err := yaml.Unmarshal(bytes, &s); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidConfigFormat
	}

	return &s, nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func isYAML(s string) bool {
	var y map[string]interface{}
	return yaml.Unmarshal([]byte(s), &y) == nil
}
