// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
	avatrace "github.com/ava-labs/avalanchego/trace"
)

type Handlers map[string]http.Handler

type Config interface {
	GetTraceConfig() *trace.Config
	GetMempoolSize() int
	GetAuthVerificationCores() int
	GetVerifyAuth() bool
	GetRootGenerationCores() int
	GetTransactionExecutionCores() int
	GetStateFetchConcurrency() int
	GetMempoolSponsorSize() int
	GetMempoolExemptSponsors() []codec.Address
	GetStreamingBacklogSize() int
	GetStateHistoryLength() int               // how many roots back of data to keep to serve state queries
	GetIntermediateNodeCacheSize() int        // how many bytes to keep in intermediate cache
	GetStateIntermediateWriteBufferSize() int // how many bytes to keep unwritten in intermediate cache
	GetStateIntermediateWriteBatchSize() int  // how many bytes to write from intermediate cache at once
	GetValueNodeCacheSize() int               // how many bytes to keep in value cache
	GetAcceptorSize() int                     // how far back we can fall in processing accepted blocks
	GetStateSyncParallelism() int
	GetStateSyncMinBlocks() uint64
	GetStateSyncServerDelay() time.Duration
	GetParsedBlockCacheSize() int
	GetAcceptedBlockWindow() int
	GetAcceptedBlockWindowCache() int
	GetContinuousProfilerConfig() *profiler.Config
	GetTargetBuildDuration() time.Duration
	GetProcessingBuildSkip() int
	GetTargetGossipDuration() time.Duration
	GetBlockCompactionFrequency() int
}

type Genesis interface {
	Load(context.Context, avatrace.Tracer, state.Mutable) error

	GetStateBranchFactor() merkledb.BranchFactor
}

type AuthEngine interface {
	GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier
	Cache(auth chain.Auth)
}

type Controller interface {
	Initialize(
		inner *VM, // hypersdk VM
		snowCtx *snow.Context,
		gatherer avametrics.MultiGatherer,
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
		authEngines map[uint8]AuthEngine,
		err error,
	)

	Rules(t int64) chain.Rules // ms

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
