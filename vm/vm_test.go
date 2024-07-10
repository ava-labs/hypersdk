// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/config"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/trace"
)

var _ Controller = (*testController)(nil)

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

	rules := chain.NewMockRules(ctrl)
	rules.EXPECT().GetValidityWindow().Return(int64(60))

	vm := VM{
		snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: metrics.NewPrefixGatherer()},
		config:  &config.Config{},

		vmDB: memdb.New(),

		tracer:                 tracer,
		acceptedBlocksByID:     bByID,
		acceptedBlocksByHeight: bByHeight,

		verifiedBlocks: make(map[ids.ID]*chain.StatelessBlock),
		seen:           emap.NewEMap[*chain.Transaction](),
		mempool:        mempool.New[*chain.Transaction](tracer, 100, 32, nil),
		acceptedQueue:  make(chan *chain.StatelessBlock, 1024), // don't block on queue
		c: &testController{
			rules: rules,
		},
	}

	// Init metrics (called in [Accepted])
	reg, m, err := newMetrics()
	require.NoError(err)
	vm.metrics = m
	require.NoError(vm.snowCtx.Metrics.Register("hypersdk", reg))

	// put the block into the cache "vm.blocks"
	// and delete from "vm.verifiedBlocks"
	ctx := context.TODO()
	vm.Accepted(ctx, blk)

	// we have not set up any persistent db
	// so this must succeed from using cache
	blk2, err := vm.GetStatelessBlock(ctx, blkID)
	require.NoError(err)
	require.Equal(blk, blk2)
}

type testController struct {
	rules chain.Rules
}

func (t *testController) Initialize(
	*VM,
	*snow.Context,
	metrics.MultiGatherer,
	[]byte,
	[]byte,
	[]byte,
) (
	config Config,
	genesis Genesis,
	builder builder.Builder,
	gossiper gossiper.Gossiper,
	vmDB database.Database,
	stateDB database.Database,
	handler Handlers,
	actionRegistry chain.ActionRegistry,
	authRegistry chain.AuthRegistry,
	authEngines map[uint8]AuthEngine,
	err error,
) {
	return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
}

func (t *testController) Rules(int64) chain.Rules {
	return t.rules
}

func (*testController) StateManager() chain.StateManager {
	return nil
}

func (*testController) Accepted(context.Context, *chain.StatelessBlock) error {
	return nil
}

func (*testController) Rejected(context.Context, *chain.StatelessBlock) error {
	return nil
}

func (*testController) Shutdown(context.Context) error {
	return nil
}
