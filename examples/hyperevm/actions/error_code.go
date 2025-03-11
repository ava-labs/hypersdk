// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import "fmt"

// The result.Err field returned by core.ApplyMessage contains an error type, but
// the actual value is not part of the EVM's state transition function. ie. if the
// error changes it should not change the state transition (block/state).
// We convert it to an error code representing the three differentiated error types:
// nil (success), revert (special case), and all other erros as a generic failure.
type ErrorCode byte

const (
	NilError ErrorCode = iota
	ErrExecutionReverted
	ErrExecutionFailed
)

func (e ErrorCode) String() string {
	switch {
	case e == NilError:
		return "nil"
	case e == ErrExecutionReverted:
		return "reverted"
	case e == ErrExecutionFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (e ErrorCode) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

func (e *ErrorCode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "nil":
		*e = NilError
	case "reverted":
		*e = ErrExecutionReverted
	case "failed":
		*e = ErrExecutionFailed
	default:
		return fmt.Errorf("failed to unmarshal error code: %s", text)
	}
	return nil
}
