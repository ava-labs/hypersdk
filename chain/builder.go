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

// TODO: move to block file?
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
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, parent, nextTime, blockContext != nil)
	epoch := utils.Epoch(nextTime, r.GetEpochDuration())

	// Attempt to add valid certs that are not expired
	b.chunks = set.NewSet[ids.ID](r.GetChunksPerBlock())
	b.AvailableChunks = make([]*ChunkCertificate, 0, r.GetChunksPerBlock()) // TODO: make this a value
	for len(b.AvailableChunks) < r.GetChunksPerBlock() {
		cert, ok := vm.NextChunkCertificate(ctx, epoch)
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

	// Fetch executed blocks
	depth := r.GetBlockExecutionDepth()
	if b.StatefulBlock.Height > depth {
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

	// TODO: put into a single log message
	if b.execHeight == nil {
		log.Info(
			"built block",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Uint64("epoch", epoch),
			zap.Stringer("parentID", b.Parent()),
			zap.Int("available chunks", len(b.AvailableChunks)),
			zap.Stringer("start root", b.StartRoot),
			zap.Int("executed chunks", len(b.ExecutedChunks)),
		)
	} else {
		log.Info(
			"built block",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Uint64("epoch", epoch),
			zap.Uint64("execHeight", *b.execHeight),
			zap.Stringer("parentID", b.Parent()),
			zap.Int("available chunks", len(b.AvailableChunks)),
			zap.Stringer("start root", b.StartRoot),
			zap.Int("executed chunks", len(b.ExecutedChunks)),
		)
	}
	return b, nil
}
