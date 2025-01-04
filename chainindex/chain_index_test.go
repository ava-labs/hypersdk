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

func (t *testBlock) ID() ids.ID     { return hashing.ComputeHash256Array(t.Bytes()) }
func (t *testBlock) Height() uint64 { return t.height }
func (t *testBlock) Bytes() []byte  { return binary.BigEndian.AppendUint64(nil, t.height) }

type parser struct{}

func (*parser) ParseBlock(_ context.Context, b []byte) (*testBlock, error) {
	if len(b) != consts.Uint64Len {
		return nil, fmt.Errorf("unexpected block length: %d", len(b))
	}
	height := binary.BigEndian.Uint64(b)
	return &testBlock{height}, nil
}

func newTestChainIndex(config Config, db database.Database) (*ChainIndex[*testBlock], error) {
	return New(logging.NoLog{}, prometheus.NewRegistry(), config, &parser{}, db)
}

func confirmBlockIndexed(r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[*testBlock], expectedBlk *testBlock) {
	blk, err := chainIndex.GetBlockByHeight(ctx, expectedBlk.height)
	r.NoError(err)
	r.Equal(expectedBlk.ID(), blk.ID())

	blkID, err := chainIndex.GetBlockIDAtHeight(ctx, expectedBlk.height)
	r.NoError(err)
	r.Equal(expectedBlk.ID(), blkID)

	blkHeight, err := chainIndex.GetBlockIDHeight(ctx, expectedBlk.ID())
	r.NoError(err)
	r.Equal(expectedBlk.Height(), blkHeight)

	blk, err = chainIndex.GetBlock(ctx, expectedBlk.ID())
	r.NoError(err)
	r.Equal(expectedBlk.ID(), blk.ID())
}

func confirmLastAcceptedHeight(r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[*testBlock], expectedHeight uint64) {
	lastAcceptedHeight, err := chainIndex.GetLastAcceptedHeight(ctx)
	r.NoError(err)
	r.Equal(expectedHeight, lastAcceptedHeight)
}

func confirmBlockUnindexed(r *require.Assertions, ctx context.Context, chainIndex *ChainIndex[*testBlock], blk *testBlock) {
	_, err := chainIndex.GetBlockByHeight(ctx, blk.height)
	r.ErrorIs(err, database.ErrNotFound)
	_, err = chainIndex.GetBlockIDAtHeight(ctx, blk.height)
	r.ErrorIs(err, database.ErrNotFound)
	_, err = chainIndex.GetBlockIDHeight(ctx, blk.ID())
	r.ErrorIs(err, database.ErrNotFound)
	_, err = chainIndex.GetBlock(ctx, blk.ID())
	r.ErrorIs(err, database.ErrNotFound)
}

func TestChainIndex(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := newTestChainIndex(NewDefaultConfig(), memdb.New())
	r.NoError(err)

	genesisBlk := &testBlock{0}
	confirmBlockUnindexed(r, ctx, chainIndex, genesisBlk)
	_, err = chainIndex.GetLastAcceptedHeight(ctx)
	r.ErrorIs(err, database.ErrNotFound)

	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.Height())

	blk1 := &testBlock{1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))
	confirmBlockIndexed(r, ctx, chainIndex, blk1)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.Height())
}

func TestChainIndexInvalidCompactionFrequency(t *testing.T) {
	_, err := newTestChainIndex(Config{BlockCompactionFrequency: 0}, memdb.New())
	require.ErrorIs(t, err, errBlockCompactionFrequencyZero)
}

func TestChainIndexExpiry(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	chainIndex, err := newTestChainIndex(Config{AcceptedBlockWindow: 1, BlockCompactionFrequency: 64}, memdb.New())
	r.NoError(err)

	genesisBlk := &testBlock{0}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, genesisBlk))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk)
	confirmLastAcceptedHeight(r, ctx, chainIndex, genesisBlk.Height())

	blk1 := &testBlock{1}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk1))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk) // Confirm genesis is not un-indexed
	confirmBlockIndexed(r, ctx, chainIndex, blk1)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk1.Height())

	blk2 := &testBlock{2}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk2))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk) // Confirm genesis is not un-indexed
	confirmBlockIndexed(r, ctx, chainIndex, blk2)
	confirmBlockUnindexed(r, ctx, chainIndex, blk1)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk2.Height())

	blk3 := &testBlock{3}
	r.NoError(chainIndex.UpdateLastAccepted(ctx, blk3))
	confirmBlockIndexed(r, ctx, chainIndex, genesisBlk) // Confirm genesis is not un-indexed
	confirmBlockIndexed(r, ctx, chainIndex, blk3)
	confirmBlockUnindexed(r, ctx, chainIndex, blk2)
	confirmLastAcceptedHeight(r, ctx, chainIndex, blk3.Height())
}
