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
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
	avatrace "github.com/ava-labs/avalanchego/trace"
)

type Handlers map[string]http.Handler

type Config struct {
	TraceConfig                      trace.Config    `json:"traceConfig"`
	MempoolSize                      int             `json:"mempoolSize"`
	AuthVerificationCores            int             `json:"authVerificationCores"`
	VerifyAuth                       bool            `json:"verifyAuth"`
	RootGenerationCores              int             `json:"rootGenerationCores"`
	TransactionExecutionCores        int             `json:"transactionExecutionCores"`
	StateFetchConcurrency            int             `json:"stateFetchConcurrency"`
	MempoolSponsorSize               int             `json:"mempoolSponsorSize"`
	StreamingBacklogSize             int             `json:"streamingBacklogSize"`
	StateHistoryLength               int             `json:"stateHistoryLength"`               // how many roots back of data to keep to serve state queries
	IntermediateNodeCacheSize        int             `json:"intermediateNodeCacheSize"`        // how many bytes to keep in intermediate cache
	StateIntermediateWriteBufferSize int             `json:"stateIntermediateWriteBufferSize"` // how many bytes to keep unwritten in intermediate cache
	StateIntermediateWriteBatchSize  int             `json:"stateIntermediateWriteBatchSize"`  // how many bytes to write from intermediate cache at once
	ValueNodeCacheSize               int             `json:"valueNodeCacheSize"`               // how many bytes to keep in value cache
	AcceptorSize                     int             `json:"acceptorSize"`                     // how far back we can fall in processing accepted blocks
	StateSyncParallelism             int             `json:"stateSyncParallelism"`
	StateSyncMinBlocks               uint64          `json:"stateSyncMinBlocks"`
	StateSyncServerDelay             time.Duration   `json:"stateSyncServerDelay"`
	ParsedBlockCacheSize             int             `json:"parsedBlockCacheSize"`
	AcceptedBlockWindow              int             `json:"acceptedBlockWindow"`
	AcceptedBlockWindowCache         int             `json:"acceptedBlockWindowCache"`
	ContinuousProfilerConfig         profiler.Config `json:"continuousProfilerConfig"`
	TargetBuildDuration              time.Duration   `json:"targetBuildDuration"`
	ProcessingBuildSkip              int             `json:"processingBuildSkip"`
	TargetGossipDuration             time.Duration   `json:"targetGossipDuration"`
	BlockCompactionFrequency         int             `json:"blockCompactionFrequency"`
	// Config is defined by the Controller
	Config map[string]any `json:"config"`
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

type ControllerFactory interface {
	New(
		inner *VM, // hypersdk VM
		snowCtx *snow.Context,
		gatherer avametrics.MultiGatherer,
		genesisBytes []byte,
		upgradeBytes []byte,
		configBytes []byte,
	) (
		Controller,
		Genesis,
		builder.Builder,
		gossiper.Gossiper,
		Handlers,
		chain.ActionRegistry,
		chain.AuthRegistry,
		map[uint8]AuthEngine,
		error,
	)
}

type Controller interface {
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
