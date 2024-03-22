// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
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

	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/config"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/trace"
)

func TestBlockCache(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// create a block with "Unknown" status
	blk := &chain.StatelessBlock{
		StatefulBlock: &chain.StatefulBlock{
			Prnt: ids.GenerateTestID(),
			Hght: 10000,
		},
	}
	blkID := blk.ID()

	tracer, _ := trace.New(&trace.Config{Enabled: false})
	bByID, _ := cache.NewFIFO[ids.ID, *chain.StatelessBlock](3)
	bByHeight, _ := cache.NewFIFO[uint64, ids.ID](3)
	controller := NewMockController(ctrl)
	vm := VM{
		snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: metrics.NewOptionalGatherer()},
		config:  &config.Config{},

		vmDB: memdb.New(),

		tracer:                 tracer,
		acceptedBlocksByID:     bByID,
		acceptedBlocksByHeight: bByHeight,

		verifiedBlocks: make(map[ids.ID]*chain.StatelessBlock),
		seen:           emap.NewEMap[*chain.Transaction](),
		mempool:        mempool.New[*chain.Transaction](tracer, 100, 32, nil),
		acceptedQueue:  make(chan *chain.StatelessBlock, 1024), // don't block on queue
		c:              controller,
	}

	// Init metrics (called in [Accepted])
	gatherer := metrics.NewMultiGatherer()
	reg, m, err := newMetrics()
	require.NoError(err)
	vm.metrics = m
	require.NoError(gatherer.Register("hypersdk", reg))
	require.NoError(vm.snowCtx.Metrics.Register(gatherer))

	// put the block into the cache "vm.blocks"
	// and delete from "vm.verifiedBlocks"
	ctx := context.TODO()
	rules := chain.NewMockRules(ctrl)
	rules.EXPECT().GetValidityWindow().Return(int64(60))
	controller.EXPECT().Rules(gomock.Any()).Return(rules)
	vm.Accepted(ctx, blk)

	// we have not set up any persistent db
	// so this must succeed from using cache
	blk2, err := vm.GetStatelessBlock(ctx, blkID)
	require.NoError(err)
	require.Equal(blk, blk2)
}
