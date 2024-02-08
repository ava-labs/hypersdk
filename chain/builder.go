// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

func BuildBlock(
	ctx context.Context,
	vm VM,
	parent *StatelessBlock,
	blockContext *smblock.Context,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	// Select next timestamp
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	if nextTime < parent.StatefulBlock.Timestamp+r.GetMinBlockGap() {
		log.Warn("block building failed", zap.Error(ErrTimestampTooEarly))
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, parent, nextTime)

	// Check block context to determine if we should add any certs
	if blockContext != nil {
		// Attempt to add valid certs that are not expired
		b.chunks = set.NewSet[ids.ID](16)
		b.AvailableChunks = make([]*ChunkCertificate, 0, 16) // TODO: make this a value
		for len(b.AvailableChunks) < 16 {
			cert, ok := vm.NextChunkCertificate(ctx)
			if !ok {
				break
			}

			// Check if the chunk is a repeat
			repeats, err := parent.IsRepeatChunk(ctx, []*ChunkCertificate{cert}, set.NewBits())
			if err != nil {
				return nil, err
			}
			if repeats.Len() > 0 {
				log.Warn("skipping duplicate chunk", zap.Stringer("chunkID", cert.Chunk))
				continue
			}

			// TODO: verify certificate signature is valid
			// TODO: verify certificate is not expired
			// TODO: verify certificate is not a repeat

			b.chunks.Add(cert.Chunk)
			b.AvailableChunks = append(b.AvailableChunks, cert)
		}
	}

	// Fetch executed blocks
	depth := r.GetBlockExecutionDepth()
	if b.StatefulBlock.Height >= depth {
		execHeight := b.StatefulBlock.Height - depth
		root, executed, err := vm.Engine().Results(execHeight)
		if err != nil {
			return nil, err
		}
		b.execHeight = &execHeight
		b.StartRoot = root
		b.ExecutedChunks = executed
	}

	// Populate all fields in block
	b.built = true
	bytes, err := b.Marshal()
	if err != nil {
		return nil, err
	}
	b.id = utils.ToID(bytes)
	b.t = time.UnixMilli(b.StatefulBlock.Timestamp)
	b.bytes = bytes
	b.parent = parent
	b.bctx = blockContext

	log.Info(
		"built block",
		zap.Stringer("blockID", b.ID()),
		zap.Uint64("height", b.StatefulBlock.Height),
		zap.Stringer("parentID", b.Parent()),
		zap.Int("available chunks", len(b.AvailableChunks)),
		zap.Stringer("start root", b.StartRoot),
		zap.Int("executed chunks", len(b.ExecutedChunks)),
	)
	return b, nil
}
