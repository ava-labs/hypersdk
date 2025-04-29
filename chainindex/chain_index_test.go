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

func newTestChainIndex(ctx context.Context, config Config, db database.Database) (*ChainIndex[*testBlock], error) {
	return New(ctx, logging.NoLog{}, prometheus.NewRegistry(), config, &parser{}, db)
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
	chainIndex, err := newTestChainIndex(ctx, NewDefaultConfig(), memdb.New())
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
	ctx := context.Background()
	_, err := newTestChainIndex(ctx, Config{BlockCompactionFrequency: 0}, memdb.New())
	require.ErrorIs(t, err, errBlockCompactionFrequencyZero)
}

func TestChainIndexExpiry(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := newTestChainIndex(ctx, Config{AcceptedBlockWindow: 1, BlockCompactionFrequency: 64}, memdb.New())
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

func TestChainIndex_SaveHistorical(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := newTestChainIndex(ctx, NewDefaultConfig(), memdb.New())
	r.NoError(err)

	// Create and save a genesis block normally first
	genesisBlk := &testBlock{height: 0}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk, nil)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.GetHeight())

	// Create a higher height block, but don't make it the last accepted
	historicalBlk := &testBlock{height: 100}

	// Save the historical block
	r.NoError(chainIndex.SaveHistorical(historicalBlk))

	// Verify the historical block is indexed
	confirmBlockIndexed(r, ctx, chainIndex, historicalBlk, nil)

	// Verify lastAccepted hasn't changed (still points to genesis)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.GetHeight())

	// Create and save a normal block that should become the new last accepted
	blk1 := &testBlock{height: 1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))

	// Verify the new block is indexed
	confirmBlockIndexed(r, ctx, chainIndex, blk1, nil)

	// Verify the historical block is still indexed
	confirmBlockIndexed(r, ctx, chainIndex, historicalBlk, nil)

	// Verify lastAccepted points to the new block
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.GetHeight())
}

func TestChainIndex_Cleanup(t *testing.T) {
	tests := []struct {
		name                   string
		config                 Config
		setupFunc              func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock])
		verifyAfterCleanupFunc func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock])
	}{
		{
			name: "If there's no accepted window, nothing to clean",
			config: Config{
				AcceptedBlockWindow:      0,
				BlockCompactionFrequency: 1,
			},
			setupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// Add blocks 0-10
				for i := 0; i <= 10; i++ {
					blkHeight := uint64(i)
					blk := &testBlock{height: blkHeight}
					if i < 5 {
						r.NoError(chainIndex.SaveHistorical(blk))
					} else {
						r.NoError(chainIndex.UpdateLastAccepted(ctx, blk))
					}
					confirmBlockIndexed(r, ctx, chainIndex, blk, nil)
				}
				confirmLastAcceptedHeight(r, ctx, chainIndex, 10)
			},
			verifyAfterCleanupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// All blocks should still exist
				for i := uint64(0); i <= 10; i++ {
					_, err := chainIndex.GetBlockByHeight(ctx, i)
					r.NoError(err, "Block at height %d should exist", i)
				}
			},
		},
		{
			name: "If lastAcceptedHeight is too small (less than window), nothing to clean",
			config: Config{
				AcceptedBlockWindow:      20,
				BlockCompactionFrequency: 1,
			},
			setupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// Add blocks 0-10
				for i := 0; i <= 10; i++ {
					blk := &testBlock{height: uint64(i)}
					if i < 5 {
						r.NoError(chainIndex.SaveHistorical(blk))
					} else {
						r.NoError(chainIndex.UpdateLastAccepted(ctx, blk))
					}
					confirmBlockIndexed(r, ctx, chainIndex, blk, nil)
				}
				confirmLastAcceptedHeight(r, ctx, chainIndex, 10)
			},
			verifyAfterCleanupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// All blocks should still exist because 10 < 20 (window)
				for i := uint64(0); i <= 10; i++ {
					_, err := chainIndex.GetBlockByHeight(ctx, i)
					r.NoError(err, "Block at height %d should exist", i)
				}
			},
		},
		{
			name: "Should cleanup historical blocks",
			config: Config{
				AcceptedBlockWindow:      5,
				BlockCompactionFrequency: 1,
			},
			setupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// Create blocks in reverse order (historical first)

				// Historical blocks (0-7) added in reverse order
				for i := 7; i >= 0; i-- {
					blk := &testBlock{height: uint64(i)}
					r.NoError(chainIndex.SaveHistorical(blk))
					confirmBlockIndexed(r, ctx, chainIndex, blk, nil)
					_, err := chainIndex.GetLastAcceptedHeight(ctx)
					r.ErrorIs(err, database.ErrNotFound)
				}

				// Last accepted blocks (8-10)
				for i := 8; i <= 10; i++ {
					blk := &testBlock{height: uint64(i)}
					// UpdateLastAccepted should clean blocks: `expiryHeight := i-AcceptedBlockWindow`
					// example: 8-5 = 3, 9-5 = 4, 10-5 = 5
					r.NoError(chainIndex.UpdateLastAccepted(ctx, blk))
					confirmBlockIndexed(r, ctx, chainIndex, blk, nil)
				}

				confirmLastAcceptedHeight(r, ctx, chainIndex, 10)
			},
			verifyAfterCleanupFunc: func(ctx context.Context, r *require.Assertions, chainIndex *ChainIndex[*testBlock]) {
				// UpdateLastAccepted deleted blocks, 3, 4 and 5 but there's gap 1 and 2, those should be deleted as well
				for i := 1; i <= 5; i++ {
					_, err := chainIndex.GetBlockByHeight(ctx, uint64(i))
					r.ErrorIs(err, database.ErrNotFound, "Block at height %d should be deleted", i)
				}

				// Blocks 6-10 should still exist
				for i := uint64(6); i <= 10; i++ {
					_, err := chainIndex.GetBlockByHeight(ctx, i)
					r.NoError(err, "Block at height %d should exist", i)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			chainIndex, err := newTestChainIndex(ctx, test.config, memdb.New())
			r.NoError(err)

			if test.setupFunc != nil {
				test.setupFunc(ctx, r, chainIndex)
			}

			r.NoError(chainIndex.cleanupOnStartup(ctx))

			if test.verifyAfterCleanupFunc != nil {
				test.verifyAfterCleanupFunc(ctx, r, chainIndex)
			}

			// Genesis should never be deleted
			genesis, err := chainIndex.GetBlockByHeight(ctx, uint64(0))
			r.Equal(uint64(0), genesis.GetHeight(), "Genesis block should not be deleted")
			r.NoError(err)
		})
	}
}
