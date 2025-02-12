// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

type testBlock struct {
	height uint64
}

func (t *testBlock) GetID() ids.ID     { return hashing.ComputeHash256Array(t.GetBytes()) }
func (t *testBlock) GetHeight() uint64 { return t.height }
func (t *testBlock) GetBytes() []byte  { return binary.BigEndian.AppendUint64(nil, t.height) }

type parser struct{}

func (*parser) ParseBlock(_ context.Context, b []byte) (*testBlock, error) {
	if len(b) != consts.Uint64Len {
		return nil, fmt.Errorf("unexpected block length: %d", len(b))
	}
	height := binary.BigEndian.Uint64(b)
	return &testBlock{height: height}, nil
}

func TestChainIndex(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	chainIndex, err := NewChainIndexTestFactory[*testBlock](&parser{}).Build()
	r.NoError(err)

	genesisBlk := &testBlock{height: 0}
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, database.ErrNotFound)
	_, err = chainIndex.GetLastAcceptedHeight(ctx)
	r.ErrorIs(err, database.ErrNotFound)

	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.GetHeight())

	blk1 := &testBlock{height: 1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))
	confirmBlockIndexed(r, ctx, chainIndex, blk1, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.GetHeight())
}

func TestChainIndexInvalidCompactionFrequency(t *testing.T) {
	_, err := NewChainIndexTestFactory(&parser{}).
		WithConfig(Config{BlockCompactionFrequency: 0}).
		Build()
	require.ErrorIs(t, err, errBlockCompactionFrequencyZero)
}

func TestChainIndexExpiry(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := NewChainIndexTestFactory[*testBlock](&parser{}).
		WithConfig(Config{
			AcceptedBlockWindow:      1,
			BlockCompactionFrequency: 64,
		}).
		Build()
	r.NoError(err)

	genesisBlk := &testBlock{height: 0}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.GetHeight())

	blk1 := &testBlock{height: 1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk1, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.GetHeight())

	blk2 := &testBlock{height: 2}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk2))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk2, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk1, database.ErrNotFound)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk2.GetHeight())

	blk3 := &testBlock{height: 3}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk3))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk3, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk2, database.ErrNotFound)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk3.GetHeight())
}

func confirmBlockIndexed[T Block](r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[T], expectedBlk T, expectedErr error) {
	blkByHeight, err := chainIndex.GetBlockByHeight(ctx, expectedBlk.GetHeight())
	r.ErrorIs(err, expectedErr)

	blkIDAtHeight, err := chainIndex.GetBlockIDAtHeight(ctx, expectedBlk.GetHeight())
	r.ErrorIs(err, expectedErr)

	blockIDHeight, err := chainIndex.GetBlockIDHeight(ctx, expectedBlk.GetID())
	r.ErrorIs(err, expectedErr)

	blk, err := chainIndex.GetBlock(ctx, expectedBlk.GetID())
	r.ErrorIs(err, expectedErr)

	if expectedErr != nil {
		return
	}

	r.Equal(blkByHeight.GetID(), expectedBlk.GetID())
	r.Equal(blkIDAtHeight, expectedBlk.GetID())
	r.Equal(blockIDHeight, expectedBlk.GetHeight())
	r.Equal(blk.GetID(), expectedBlk.GetID())
}

func confirmLastAcceptedHeight[T Block](r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[T], expectedHeight uint64) {
	lastAcceptedHeight, err := chainIndex.GetLastAcceptedHeight(ctx)
	r.NoError(err)
	r.Equal(expectedHeight, lastAcceptedHeight)
}
