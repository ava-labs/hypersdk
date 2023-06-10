// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	atrace "github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/profiler"

	"github.com/AnomalyFi/hypersdk/builder"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/gossiper"
	trace "github.com/AnomalyFi/hypersdk/trace"
)

type Handlers map[string]*common.HTTPHandler

type Config interface {
	GetTraceConfig() *trace.Config
	GetParallelism() int // how many cores to use during verification
	GetMempoolSize() int
	GetMempoolPayerSize() int
	GetMempoolExemptPayers() [][]byte
	GetMempoolVerifyBalances() bool
	GetStreamingBacklogSize() int
	GetStateHistoryLength() int // how many roots back of data to keep to serve state queries
	GetStateCacheSize() int     // how many items to keep in value cache and node cache
	GetAcceptorSize() int       // how far back we can fall in processing accepted blocks
	GetStateSyncParallelism() int
	GetStateSyncMinBlocks() uint64
	GetStateSyncServerDelay() time.Duration
	GetBlockLRUSize() int
	GetContinuousProfilerConfig() *profiler.Config
}

type Genesis interface {
	GetHRP() string
	Load(context.Context, atrace.Tracer, chain.Database) error
}

type Controller interface {
	Initialize(
		inner *VM, // hypersdk VM
		snowCtx *snow.Context,
		gatherer ametrics.MultiGatherer,
		genesisBytes []byte,
		upgradeBytes []byte,
		configBytes []byte,
	) (
		config Config,
		genesis Genesis,
		builder builder.Builder,
		gossiper gossiper.Gossiper,
		// TODO: consider splitting out blockDB for use with more experimental
		// databases
		vmDB database.Database,
		stateDB database.Database,
		handler Handlers,
		actionRegistry chain.ActionRegistry,
		authRegistry chain.AuthRegistry,
		err error,
	)

	Rules(t int64) chain.Rules

	// StateManager is used by the VM to request keys to store required
	// information in state (without clobbering things the Controller is
	// storing).
	StateManager() chain.StateManager

	// Anything that the VM wishes to store outside of state or blocks must be
	// recorded here
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
	Rejected(ctx context.Context, blk *chain.StatelessBlock) error

	// Shutdown should be used by the [Controller] to terminate any async
	// processes it may be running in the background. It is invoked when
	// `vm.Shutdown` is called.
	Shutdown(context.Context) error
}
