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

type Block struct {
	StatelessChunkBlock

	backend Backend
}

func (b *Block) Verify(ctx context.Context) error {
	for _, chunkCertificate := range b.Chunks {
		if err := chunkCertificate.Verify(ctx); err != nil {
			return err
		}
	}

	return b.backend.Verify(ctx, b)
}

func (b *Block) Accept(ctx context.Context) error {
	return b.backend.Accept(ctx, b)
}

func (b *Block) Reject(ctx context.Context) error {
	return b.backend.Reject(ctx, b)
}
