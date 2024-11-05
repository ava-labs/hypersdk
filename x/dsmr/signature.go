// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

var _ ChunkVerifier[Tx] = (*NoOpChunkVerifier[Tx])(nil)

type ChunkVerifier[T Tx] interface {
	VerifyChunk(
		ctx context.Context,
		chunk Chunk[T],
		signature warp.Signature,
		sourceChainID ids.ID,
		pchainState validators.State,
	) error
}

type NoOpChunkVerifier[T Tx] struct{}

func (*NoOpChunkVerifier[T]) VerifyChunk(context.Context, Chunk[T], warp.Signature) error {}

type WarpChunkVerifier[T Tx] struct {
	NetworkID uint32
	ChainID   ids.ID
}

func (w *WarpChunkVerifier[T]) VerifyChunk(
	ctx context.Context,
	chunk Chunk[T],
	signature warp.Signature,
) error {
	msg, err := warp.NewUnsignedMessage(w.NetworkID, w.ChainID, chunk.bytes)
	if err != nil {
		return fmt.Errorf("failed to initialize warp message")
	}

	if err := signature.Verify(ctx, msg, w.NetworkID, w.pChainState, w.pChainHeight, w.quorumNum, w.quorumDen); err != nil {
		return fmt.Errorf("failed warp verification: %w", err)
	}

	return nil
}
