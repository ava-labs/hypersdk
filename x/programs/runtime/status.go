// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "fmt"

// The runtime uses the following status codes:
//
//    Code           | Number | Description
//    ---------------|--------|------------------------------------
//    OK             |  0     | No error occurred.
//    Unknown        | -1     | Unknown error does not contain enough information to report why the error occurred.
//    NotFound       | -2     | The requested item could not be found.
//    InvalidBalance | -3     | The account balance is invalid.
//
// These codes are written to memory in the following format
// 1st byte: status code BigEndian
type Status int

const (
	StatusOK Status = 0
	StatusCodeUnknown Status = -1
    StatusInvalidBalance Status = -2
	StatusNotFound Status = -3
)

// String returns a string representation of the status code.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusCodeUnknown:
		return "Unknown"
	case StatusInvalidBalance:
		return "InvalidBalance"
	case StatusNotFound:
		return "NotFound"
	default:
		panic(fmt.Sprintf("unknown status code %d", s))
	}
}

type HostResult struct {
	Value int64
	Status *Status
}

