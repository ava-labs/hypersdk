// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import "context"

// TODO delete this file
// TODO: implement VM
type VM[VerificationContext any] interface {
	Verify(context.Context, *ExecutionBlock[VerificationContext]) error
	Accept(context.Context, *ExecutionBlock[VerificationContext]) error
	Reject(context.Context, *ExecutionBlock[VerificationContext]) error
}
