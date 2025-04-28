// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
)

var (
	_ block.ChainVM                      = (*SnowVM[Block, Block, Block])(nil)
	_ block.StateSyncableVM              = (*SnowVM[Block, Block, Block])(nil)
	_ block.BuildBlockWithContextChainVM = (*SnowVM[Block, Block, Block])(nil)
)

// SnowVM wraps the snow.VM and completes the implementation of block.ChainVM by providing
// alternative block handler functions that provide the snowman.Block type to the
// consensus engine.
//
//nolint:revive
type SnowVM[I Block, O Block, A Block] struct {
	*VM[I, O, A]
}

// NewSnowVM wraps snow.VM and returns SnowVM
func NewSnowVM[I Block, O Block, A Block](version string, chain Chain[I, O, A]) *SnowVM[I, O, A] {
	return &SnowVM[I, O, A]{VM: NewVM(version, chain)}
}

// GetBlock calls the VM.GetBlock and returns the snowman.Block type satisfying block.ChainVM
func (v *SnowVM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (snowman.Block, error) {
	return v.VM.GetBlock(ctx, blkID)
}

// ParseBlock calls the VM.ParseBlock and returns the snowman.Block type satisfying block.ChainVM
func (v *SnowVM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (snowman.Block, error) {
	return v.VM.ParseBlock(ctx, bytes)
}

// BuildBlock calls the VM.BuildBlock and returns the snowman.Block type satisfying block.ChainVM
func (v *SnowVM[I, O, A]) BuildBlock(ctx context.Context) (snowman.Block, error) {
	return v.VM.BuildBlock(ctx)
}

// BuildBlockWithContext calls the VM.BuildBlockWithContext and returns the snowman.Block type satisfying block.BuildBlockWithContextChainVM
func (v *SnowVM[I, O, A]) BuildBlockWithContext(ctx context.Context, blockContext *block.Context) (snowman.Block, error) {
	return v.VM.BuildBlockWithContext(ctx, blockContext)
}
