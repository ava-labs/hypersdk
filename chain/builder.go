// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
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

	// We don't need to fetch the [VerifyContext] because
	// we will always have a block to build on.

	// Select next timestamp
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	if nextTime < parent.StatefulBlock.Timestamp+r.GetMinBlockGap() {
		log.Debug("block building failed", zap.Error(ErrTimestampTooEarly))
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, parent, nextTime)

	// Check block context to determine if we should add any certs
	if blockContext != nil {
		b.bctx = blockContext

		// Attempt to add valid certs that are not expired
		b.AvailableChunks = make([]*ChunkCertificate, 0, 16) // TODO: make this a value
		for len(b.AvailableChunks) < 16 {
			cert, ok := vm.NextChunkCertificate(ctx)
			if !ok {
				break
			}

			// TODO: verify certificate signature is valid
			// TODO: verify certificate is not expired
			// TODO: verify certificate is not a repeat

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
		b.StartRoot = root
		b.ExecutedChunks = executed
	}

	return b, nil
}
