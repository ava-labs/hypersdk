// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v2"
)

const (
	ProgramCreate  = "program_create"
	ProgramExecute = "execute"
)

type Plan struct {
	// The name of the plan.
	Name string `json,yaml:"name"`
	// A description of the plan.
	Description string `json,yaml:"description"`
	// The key of the caller used in each step of the plan.
	CallerKey string `yaml:"caller_key" json:"callerKey"`
	// Steps to performed during simulation.
	Steps []Step `json,yaml:"steps"`
}

type Step struct {
	// Description of the step. (required)
	Description string `json,yaml:"description"`
	// The API endpoint to call. (required)
	Endpoint Endpoint `json,yaml:"endpoint"`
	// The method to call on the endpoint.
	Method string `json,yaml:"method"`
	// The parameters to pass to the method.
	Params []Parameter `json,yaml:"params"`
	// Define required assertions against this step.
	Require Require `json,yaml:"require,omitempty"`
}

type Endpoint string

const (
	/// Perform an operation against the key api.
	KeyEndpoint Endpoint = "key"
	/// Make a read-only call to a program function and return the result.
	ReadOnlyEndpoint Endpoint = "readonly"
	/// Create a transaction on-chain from a possible state changing program
	/// function call. A program's function can internally optionally call other
	/// functions including program to program.
	ExecuteEndpoint Endpoint = "execute"
)

func NewResponse(id int) *Response {
	return &Response{
		ID: id,
	}
}

type Response struct {
	// The index of the step that generated this response.
	ID int `json,yaml:"id"`
	// The result of the step.
	Result Result `json,yaml:"result,omitempty"`
	// The error message if available.
	Error string `json,yaml:"error,omitempty"`
}

func (r *Response) Print() {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		fmt.Printf(`{"error": "failed to marshal response"}`)
	}

	// print response
	fmt.Println(string(jsonBytes))
}

func (r *Response) Err(err error) error {
	r.Error = err.Error()
	r.Print()
	return nil
}

type Result struct {
	// The tx id of the transaction that was created.
	ID string `json,yaml:"id,omitempty"`
	// The balance after the step has completed.
	Balance uint64 `json,yaml:"balance,omitempty"`
	// The result of the call.
	Response []uint64 `json,yaml:"response,omitempty"`
	// An optional message.
	Msg string `json,yaml:"msg,omitempty"`
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
	// The optional name of the parameter. This is only used for readability.
	Name string `json,yaml:"name,omitempty"`
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
func validateAssertion(actual uint64, assertion *ResultAssertion) bool {
	// convert the assertion value(string) to uint64
	value, err := strconv.ParseUint(assertion.Value, 10, 64)
	if err != nil {
		panic(err)
	}

	switch Operator(assertion.Operator) {
	case NumericGt:
		if actual > value {
			return true
		}
	case NumericLt:
		if actual < value {
			return true
		}
	case NumericGe:
		if actual >= value {
			return true
		}
	case NumericLe:
		if actual <= value {
			return true
		}
	case NumericEq:
		if actual == value {
			return true
		}
	case NumericNe:
		if actual != value {
			return true
		}
	}

	return false
}

func unmarshalPlan(bytes []byte) (*Plan, error) {
	var p Plan
	switch {
	case isJSON(string(bytes)):
		if err := json.Unmarshal(bytes, &p); err != nil {
			return nil, err
		}
	case isYAML(string(bytes)):
		if err := yaml.Unmarshal(bytes, &p); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidConfigFormat
	}

	return &p, nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func intToUint64(i interface{}) (uint64, error) {
	int, ok := i.(int)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, Uint64)
	}
	if int < 0 {
		return 0, fmt.Errorf("failed to convert negative integer to uint64")
	}
	return uint64(int), nil
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func isYAML(s string) bool {
	var y map[string]interface{}
	return yaml.Unmarshal([]byte(s), &y) == nil
}
