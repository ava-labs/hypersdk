// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"testing"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/AnomalyFi/hypersdk/mempool"
	"github.com/AnomalyFi/hypersdk/trace"
)

func TestBlockCache(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// create a block with "Unknown" status
	blk := &chain.StatelessBlock{
		StatefulBlock: &chain.StatefulBlock{
			Prnt:      ids.GenerateTestID(),
			Hght:      10000,
			UnitPrice: 1000,
			BlockCost: 100,
		},
	}
	blkID := blk.ID()

	tracer, _ := trace.New(&trace.Config{Enabled: false})
	controller := NewMockController(ctrl)
	vm := VM{
		snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: ametrics.NewOptionalGatherer()},

		tracer: tracer,

		blocks:         &cache.LRU[ids.ID, *chain.StatelessBlock]{Size: 3},
		verifiedBlocks: make(map[ids.ID]*chain.StatelessBlock),
		seen:           emap.NewEMap[*chain.Transaction](),
		mempool:        mempool.New[*chain.Transaction](tracer, 100, 32, nil),
		acceptedQueue:  make(chan *chain.StatelessBlock, 1024), // don't block on queue
		c:              controller,
	}

	// Init metrics (called in [Accepted])
	gatherer := ametrics.NewMultiGatherer()
	reg, m, err := newMetrics()
	require.NoError(err)
	vm.metrics = m
	require.NoError(gatherer.Register("hyper_sdk", reg))
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
