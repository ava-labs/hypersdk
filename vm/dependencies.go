// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"net/http"
	"time"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/snow"
	atrace "github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	trace "github.com/ava-labs/hypersdk/trace"
)

type Handlers map[string]http.Handler

type Config interface {
	GetTraceConfig() *trace.Config
	GetMempoolSize() int
	GetAuthExecutionCores() int
	GetAuthRPCCores() int
	GetAuthRPCBacklog() int
	GetAuthGossipCores() int
	GetAuthGossipBacklog() int
	GetVerifyAuth() bool
	GetPrecheckCores() int
	GetActionExecutionCores() int
	GetMissingChunkFetchers() int
	GetChunkStorageCores() int
	GetChunkStorageBacklog() int
	GetBeneficiary() codec.Address
	GetMempoolSponsorSize() int
	GetMempoolExemptSponsors() []codec.Address
	GetStreamingBacklogSize() int
	GetAcceptorSize() int // how far back we can fall in processing accepted blocks
	GetParsedBlockCacheSize() int
	GetAcceptedBlockWindow() int
	GetAcceptedBlockWindowCache() int
	GetContinuousProfilerConfig() *profiler.Config
	GetTargetChunkBuildDuration() time.Duration
	GetChunkBuildFrequency() time.Duration
	GetBlockBuildFrequency() time.Duration
	GetProcessingBuildSkip() int
	GetMinimumCertificateBroadcastNumerator() uint64
	GetAnchorURL() string
}

type Genesis interface {
	Load(context.Context, atrace.Tracer, state.Mutable) error

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
		gatherer ametrics.MultiGatherer,
		genesisBytes []byte,
		upgradeBytes []byte,
		configBytes []byte,
	) (
		config Config,
		genesis Genesis,
		handler Handlers, // TODO: remove
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
