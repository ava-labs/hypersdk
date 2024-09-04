// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/internal/trace"
)

func TestBlockCache(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// create a block with "Unknown" status
	blk := &chain.StatefulBlock{
		StatelessBlock: &chain.StatelessBlock{
			Prnt: ids.GenerateTestID(),
			Hght: 10000,
		},
	}
	blkID := blk.ID()

	tracer, _ := trace.New(&trace.Config{Enabled: false})
	bByID, _ := cache.NewFIFO[ids.ID, *chain.StatefulBlock](3)
	bByHeight, _ := cache.NewFIFO[uint64, ids.ID](3)
	rules := chain.NewMockRules(ctrl)
	vm := VM{
		snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: metrics.NewPrefixGatherer()},
		config:  NewConfig(),
		vmDB:    memdb.New(),

		tracer:                 tracer,
		acceptedBlocksByID:     bByID,
		acceptedBlocksByHeight: bByHeight,

		verifiedBlocks: make(map[ids.ID]*chain.StatefulBlock),
		seen:           emap.NewEMap[*chain.Transaction](),
		mempool:        mempool.New[*chain.Transaction](tracer, 100, 32),
		acceptedQueue:  make(chan *chain.StatefulBlock, 1024), // don't block on queue
		ruleFactory:    &genesis.ImmutableRuleFactory{Rules: rules},
	}

	// Init metrics (called in [Accepted])
	reg, m, err := newMetrics()
	require.NoError(err)
	vm.metrics = m
	require.NoError(vm.snowCtx.Metrics.Register("hypersdk", reg))

	// put the block into the cache "vm.blocks"
	// and delete from "vm.verifiedBlocks"
	ctx := context.TODO()
	rules.EXPECT().GetValidityWindow().Return(int64(60))
	vm.Accepted(ctx, blk)

	// we have not set up any persistent db
	// so this must succeed from using cache
	blk2, err := vm.GetStatefulBlock(ctx, blkID)
	require.NoError(err)
	require.Equal(blk, blk2)
}
