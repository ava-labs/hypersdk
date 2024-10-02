package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type StatelessChunkBlock struct {
	ParentID    ids.ID `serialize:"true"`
	BlockHeight uint64 `serialize:"true"`
	Time        int64  `serialize:"true"`

	Chunks []*ChunkCertificate `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func (b *StatelessChunkBlock) ID() ids.ID {
	return b.id
}

func (b *StatelessChunkBlock) Bytes() []byte {
	return b.bytes
}

func (b *StatelessChunkBlock) Height() uint64 {
	return b.BlockHeight
}

func (b *StatelessChunkBlock) Timestamp() int64 {
	return b.Time
}

func (b *StatelessChunkBlock) Parent() ids.ID {
	return b.ParentID
}

type ExecutionBlock struct {
	StatelessChunkBlock

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
