// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/pebble"
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

func newTestChainIndex(t *testing.T, config Config) (*ChainIndex[*testBlock], error) {
	db, err := pebble.New(t.TempDir(), pebble.NewDefaultConfig(), prometheus.NewRegistry())
	if err != nil {
		return nil, err
	}
	return New(logging.NoLog{}, prometheus.NewRegistry(), config, &parser{}, db)
}

func confirmBlockIndexed(r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[*testBlock], expectedBlk *testBlock, expectedErr error) {
	blkByHeight, err := chainIndex.GetBlockByHeight(ctx, expectedBlk.height)
	r.ErrorIs(err, expectedErr)

	blkIDAtHeight, err := chainIndex.GetBlockIDAtHeight(ctx, expectedBlk.height)
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

func confirmLastAcceptedHeight(r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[*testBlock], expectedHeight uint64) {
	lastAcceptedHeight, err := chainIndex.GetLastAcceptedHeight(ctx)
	r.NoError(err)
	r.Equal(expectedHeight, lastAcceptedHeight)
}

func TestChainIndex(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := newTestChainIndex(t, NewDefaultConfig())
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
	_, err := New(logging.NoLog{}, prometheus.NewRegistry(), Config{BlockCompactionFrequency: 0}, &parser{}, memdb.New())
	require.ErrorIs(t, err, errBlockCompactionFrequencyZero)
}

func TestChainIndexExpiry(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	chainIndex, err := newTestChainIndex(t, Config{AcceptedBlockWindow: 1, BlockCompactionFrequency: 64})
	r.NoError(err)

	genesisBlk := &testBlock{height: 0}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.GetHeight())

	blk1 := &testBlock{height: 1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil) // Confirm genesis is not un-indexed, nild
	confirmBlockIndexed(r, ctx, chainIndex, blk1, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.GetHeight())

	blk2 := &testBlock{height: 2}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk2))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil) // Confirm genesis is not un-indexed, nild
	confirmBlockIndexed(r, ctx, chainIndex, blk2, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk1, database.ErrNotFound)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk2.GetHeight())

	blk3 := &testBlock{height: 3}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk3))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil) // Confirm genesis is not un-indexed, nild
	confirmBlockIndexed(r, ctx, chainIndex, blk3, nil)
	confirmBlockIndexed(r, ctx, chainIndex, blk2, database.ErrNotFound)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk3.GetHeight())
}

// TestChainIndexPruning verifies that UpdateLastAccepted properly prunes blocks outside
// the accepted window while preserving the genesis block. It simulates a scenario where
// multiple blocks are written before acceptance to ensure the pruning logic maintains
// the correct window invariant
func TestChainIndexPruning(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	acceptedBlockWindow := uint64(5)
	chainIndex, err := newTestChainIndex(t, Config{AcceptedBlockWindow: acceptedBlockWindow, BlockCompactionFrequency: 64})
	r.NoError(err)

	// Write genesis and verify initial state
	genesisBlk := &testBlock{height: 0}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, 0)

	// Write blocks up to window size + 1
	var lastBlock *testBlock
	for i := uint64(1); i <= acceptedBlockWindow+1; i++ {
		blk := &testBlock{height: i}
		r.NoError(chainIndex.WriteBlock(blk))
		confirmBlockIndexed(r, ctx, chainIndex, blk, nil)
		lastBlock = blk
	}

	// Accept new block and verify pruning behavior
	newBlk := &testBlock{height: lastBlock.GetHeight() + 1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, newBlk))

	// Verify genesis and recent blocks exist
	verifyState := func(height uint64, wantErr error) {
		expectedBlk := &testBlock{height: height}
		blk, err := chainIndex.GetBlock(ctx, expectedBlk.GetID())
		r.ErrorIs(err, wantErr)

		blkID, err := chainIndex.GetBlockIDAtHeight(ctx, expectedBlk.GetHeight())
		r.ErrorIs(err, wantErr)

		blkHeight, err := chainIndex.GetBlockIDHeight(ctx, expectedBlk.GetID())
		r.ErrorIs(err, wantErr)

		if wantErr == nil {
			r.Equal(expectedBlk.GetHeight(), blk.GetHeight())
			r.Equal(expectedBlk.GetID(), blkID)
			r.Equal(expectedBlk.GetHeight(), blkHeight)
		}
	}

	// We have written 7 blocks, our window includes 5 blocks, we should not have 1 and 2
	verifyState(0, nil)                  // Genesis
	verifyState(1, database.ErrNotFound) // Pruned
	verifyState(2, database.ErrNotFound) // Pruned
	for i := uint64(3); i <= newBlk.GetHeight(); i++ {
		verifyState(i, nil)
	}
}
