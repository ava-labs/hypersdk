package dsmr

import "context"

// TODO: implement Backend
type Backend interface {
	Verify(context.Context, *Block) error
	Accept(context.Context, *Block) error
	Reject(context.Context, *Block) error
}
