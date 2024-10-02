package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type Block struct {
	ID          ids.ID `serialize:"true"`
	ParentID    ids.ID `serialize:"true"`
	BlockHeight uint64 `serialize:"true"`
	Time        int64  `serialize:"true"`

	Chunks []*ChunkCertificate `serialize:"true"`

	bytes []byte
}

type ExecutionBlock struct {
	Block

	backend Backend
}

func (e *ExecutionBlock) Verify(ctx context.Context) error {
	for _, chunkCertificate := range e.Chunks {
		if err := chunkCertificate.Verify(ctx); err != nil {
			return err
		}
	}

	return e.backend.Verify(ctx, e)
}

func (e *ExecutionBlock) Accept(ctx context.Context) error {
	return e.backend.Accept(ctx, e)
}

func (e *ExecutionBlock) Reject(ctx context.Context) error {
	return e.backend.Reject(ctx, e)
}
