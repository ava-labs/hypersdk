// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/utils/units"
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

type Config struct {
	TraceConfig                      trace.Config
	MempoolSize                      int
	AuthVerificationCores            int
	VerifyAuth                       bool
	RootGenerationCores              int
	TransactionExecutionCores        int
	StateFetchConcurrency            int
	MempoolSponsorSize               int
	MempoolExemptSponsors            []codec.Address
	StreamingBacklogSize             int
	StateHistoryLength               int // how many roots back of data to keep to serve state queries
	IntermediateNodeCacheSize        int // how many bytes to keep in intermediate cache
	StateIntermediateWriteBufferSize int // how many bytes to keep unwritten in intermediate cache
	StateIntermediateWriteBatchSize  int // how many bytes to write from intermediate cache at once
	ValueNodeCacheSize               int // how many bytes to keep in value cache
	AcceptorSize                     int // how far back we can fall in processing accepted blocks
	StateSyncParallelism             int
	StateSyncMinBlocks               uint64
	StateSyncServerDelay             time.Duration
	ParsedBlockCacheSize             int
	AcceptedBlockWindow              int
	AcceptedBlockWindowCache         int
	ContinuousProfilerConfig         profiler.Config
	TargetBuildDuration              time.Duration
	ProcessingBuildSkip              int
	TargetGossipDuration             time.Duration
	BlockCompactionFrequency         int
}

func NewConfig() Config {
	return Config{
		TraceConfig:                      trace.Config{Enabled: false},
		MempoolSize:                      2_048,
		AuthVerificationCores:            1,
		VerifyAuth:                       true,
		RootGenerationCores:              1,
		TransactionExecutionCores:        1,
		StateFetchConcurrency:            1,
		MempoolSponsorSize:               32,
		MempoolExemptSponsors:            nil,
		StreamingBacklogSize:             1_024,
		StateHistoryLength:               256,
		IntermediateNodeCacheSize:        4 * units.GiB,
		StateIntermediateWriteBufferSize: 32 * units.MiB,
		StateIntermediateWriteBatchSize:  4 * units.MiB,
		ValueNodeCacheSize:               2 * units.GiB,
		AcceptorSize:                     64,
		StateSyncParallelism:             4,
		StateSyncMinBlocks:               768, // set to max int for archive nodes to ensure no skips
		StateSyncServerDelay:             0,   // used for testing
		ParsedBlockCacheSize:             128,
		AcceptedBlockWindow:              50_000, // ~3.5hr with 250ms block time (100GB at 2MB)
		AcceptedBlockWindowCache:         128,    // 256MB at 2MB blocks
		ContinuousProfilerConfig:         profiler.Config{Enabled: false},
		TargetBuildDuration:              100 * time.Millisecond,
		ProcessingBuildSkip:              16,
		TargetGossipDuration:             20 * time.Millisecond,
		BlockCompactionFrequency:         32, // 64 MB of deletion if 2 MB blocks
	}
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

	// Shutdown should be used by the [Controller] to terminate any async
	// processes it may be running in the background. It is invoked when
	// `vm.Shutdown` is called.
	Shutdown(context.Context) error
}
