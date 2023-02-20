// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"io"
	"time"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	atrace "github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	trace "github.com/ava-labs/hypersdk/trace"
)

type Handlers map[string]*common.HTTPHandler

type Config interface {
	GetTraceConfig() *trace.Config
	GetParallelism() int // how many cores to use during verification
	GetMempoolSize() int
	GetMempoolPayerSize() int
	GetMempoolExemptPayers() [][]byte
	GetDecisionsPort() uint16
	GetBlocksPort() uint16
	GetStreamingBacklogSize() int
	GetStateHistoryLength() int // how many roots back of data to keep to serve state queries
	GetStateCacheSize() int     // how many items to keep in value cache and node cache
	GetAcceptorSize() int       // how far back we can fall in processing accepted blocks
	GetStateSyncParallelism() int
	GetStateSyncMinBlocks() uint64
	GetStateSyncServerDelay() time.Duration
	GetBlockLRUSize() int
}

type Genesis interface {
	GetHRP() string
	Load(context.Context, atrace.Tracer, chain.Database) error
}

// Limited required functionality makes it much easier to experiment with
// unique mechanisms here (ideally ones that don't trigger compaction)
type KVDatabase interface {
	database.KeyValueReaderWriterDeleter
	io.Closer
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
		blockDB KVDatabase,
		stateDB database.Database,
		handler Handlers,
		actionRegistry chain.ActionRegistry,
		authRegistry chain.AuthRegistry,
		err error,
	)
	Rules(t int64) chain.Rules

	// Anything that the VM wishes to store outside of state or blocks must be
	// recorded here
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
	Rejected(ctx context.Context, blk *chain.StatelessBlock) error

	// TODO: add a Close function that can be used to shutdown anything the VM is
	// not aware of
}
