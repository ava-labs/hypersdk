package dsmr

import "context"

// TODO: implement VM
type VM interface {
	Verify(context.Context, *ExecutionBlock) error
	Accept(context.Context, *ExecutionBlock) error
	Reject(context.Context, *ExecutionBlock) error
}
