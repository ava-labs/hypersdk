package dsmr

import "context"

// TODO: implement Backend
type Backend interface {
	Verify(context.Context, *ExecutionBlock) error
	Accept(context.Context, *ExecutionBlock) error
	Reject(context.Context, *ExecutionBlock) error
}
