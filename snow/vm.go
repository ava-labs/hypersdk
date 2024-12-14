// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/ava-labs/avalanchego/api/health"
	avacache "github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	hcontext "github.com/ava-labs/hypersdk/context"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/lifecycle"
	"go.uber.org/zap"
)

var (
	_ block.ChainVM         = (*VM[Block, Block, Block])(nil)
	_ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)
)

type ChainInput struct {
	SnowCtx                                 *snow.Context
	GenesisBytes, UpgradeBytes, ConfigBytes []byte
	ToEngine                                chan<- common.Message
	Shutdown                                <-chan struct{}
	Context                                 *hcontext.Context
}

type Chain[I Block, O Block, A Block] interface {
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		chainIndex ChainIndex[I, O, A],
		options *Options[I, O, A],
	) (BlockChainIndex[I], O, A, bool, error)
	BuildBlock(ctx context.Context, parent O) (I, O, error)
	ParseBlock(ctx context.Context, bytes []byte) (I, error)
	Execute(
		ctx context.Context,
		parent O,
		block I,
	) (O, error)
	AcceptBlock(ctx context.Context, verifiedBlock O) (A, error)
}

type VM[I Block, O Block, A Block] struct {
	chain       Chain[I, O, A]
	chainIndex  BlockChainIndex[I]
	covariantVM *CovariantVM[I, O, A]
	Options     Options[I, O, A]

	snowCtx *snow.Context

	vmConfig VMConfig
	hctx     *hcontext.Context
	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *avacache.LRU[ids.ID, *StatefulBlock[I, O, A]]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL      sync.RWMutex
	verifiedBlocks map[ids.ID]*StatefulBlock[I, O, A]

	// We store the last [AcceptedBlockWindowCache] blocks in memory
	// to avoid reading blocks from disk.
	acceptedBlocksByID     *cache.FIFO[ids.ID, *StatefulBlock[I, O, A]]
	acceptedBlocksByHeight *cache.FIFO[uint64, ids.ID]

	lastAcceptedBlock *StatefulBlock[I, O, A]
	preferredBlkID    ids.ID

	metrics *Metrics
	log     logging.Logger
	tracer  trace.Tracer

	shutdownChan chan struct{}
}

func NewVM[I Block, O Block, A Block](chain Chain[I, O, A]) *VM[I, O, A] {
	v := &VM[I, O, A]{
		chain: chain,
		Options: Options[I, O, A]{
			Version: "v0.0.1",
			HealthChecker: health.CheckerFunc(func(ctx context.Context) (interface{}, error) {
				return nil, nil
			}),
			Ready:    lifecycle.NewAtomicBoolReady(true),
			Handlers: make(map[string]http.Handler),
		},
	}
	v.Options.vm = v
	return v
}

func (v *VM[I, O, A]) Initialize(
	ctx context.Context,
	chainCtx *snow.Context,
	db database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender,
) error {
	v.snowCtx = chainCtx
	v.covariantVM = &CovariantVM[I, O, A]{v}
	v.shutdownChan = make(chan struct{})

	hctx, err := hcontext.New(
		chainCtx.Log,
		chainCtx.Metrics,
		configBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to create hypersdk context: %w", err)
	}
	v.hctx = hctx
	v.tracer = hctx.Tracer()
	ctx, span := v.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	v.vmConfig, err = GetVMConfig(v.hctx)
	if err != nil {
		return fmt.Errorf("failed to parse vm config: %w", err)
	}

	defaultRegistry, err := v.hctx.MakeRegistry("hypersdk")
	if err != nil {
		return err
	}
	metrics, err := newMetrics(defaultRegistry)
	if err != nil {
		return err
	}
	v.metrics = metrics
	v.log = chainCtx.Log

	continuousProfilerConfig, err := GetProfilerConfig(v.hctx)
	if err != nil {
		return fmt.Errorf("failed to parse continuous profiler config: %w", err)
	}
	if continuousProfilerConfig.Enabled {
		continuousProfiler := profiler.NewContinuous(
			continuousProfilerConfig.Dir,
			continuousProfilerConfig.Freq,
			continuousProfilerConfig.MaxNumFiles,
		)
		v.Options.WithCloser(func() error {
			continuousProfiler.Shutdown()
			return nil
		})
		go continuousProfiler.Dispatch() //nolint:errcheck
	}

	v.Options.Network, err = p2p.NewNetwork(v.log, appSender, defaultRegistry, "p2p")
	if err != nil {
		return fmt.Errorf("failed to initialize p2p: %w", err)
	}

	acceptedBlocksByIDCache, err := cache.NewFIFO[ids.ID, *StatefulBlock[I, O, A]](v.vmConfig.AcceptedBlockWindowCache)
	if err != nil {
		return err
	}
	v.acceptedBlocksByID = acceptedBlocksByIDCache
	acceptedBlocksByHeightCache, err := cache.NewFIFO[uint64, ids.ID](v.vmConfig.AcceptedBlockWindowCache)
	if err != nil {
		return err
	}
	v.acceptedBlocksByHeight = acceptedBlocksByHeightCache
	v.parsedBlocks = &avacache.LRU[ids.ID, *StatefulBlock[I, O, A]]{Size: v.vmConfig.ParsedBlockCacheSize}
	v.verifiedBlocks = make(map[ids.ID]*StatefulBlock[I, O, A])

	chainInput := ChainInput{
		SnowCtx:      chainCtx,
		GenesisBytes: genesisBytes,
		UpgradeBytes: upgradeBytes,
		ConfigBytes:  configBytes,
		ToEngine:     toEngine,
		Shutdown:     v.shutdownChan,
		Context:      v.hctx,
	}

	blockChainIndex, outputBlock, acceptedBlock, stateReady, err := v.chain.Initialize(
		ctx,
		chainInput,
		ChainIndex[I, O, A]{covariantVM: v.covariantVM},
		&v.Options,
	)
	if err != nil {
		return err
	}
	v.chainIndex = blockChainIndex
	lastAcceptedHeight, err := v.chainIndex.GetLastAcceptedHeight(ctx)
	if err != nil {
		return err
	}
	inputBlock, err := v.chainIndex.GetBlockByHeight(ctx, lastAcceptedHeight)
	if err != nil {
		return err
	}
	var lastAcceptedBlock *StatefulBlock[I, O, A]
	if stateReady {
		lastAcceptedBlock, err = v.reprocessToLastAccepted(ctx, inputBlock, outputBlock, acceptedBlock)
		if err != nil {
			return err
		}
	} else {
		lastAcceptedBlock = NewInputBlock(v.covariantVM, inputBlock)
		v.Options.Ready.MarkNotReady()
	}
	v.setLastAccepted(lastAcceptedBlock)
	return nil
}

func (v *VM[I, O, A]) reprocessToLastAccepted(ctx context.Context, inputBlock I, outputBlock O, acceptedBlock A) (*StatefulBlock[I, O, A], error) {
	if inputBlock.Height() < outputBlock.Height() || outputBlock.Height() < acceptedBlock.Height() {
		return nil, fmt.Errorf("invalid initial accepted state (Input = %s, Output = %s, Accepted = %s)", inputBlock, outputBlock, acceptedBlock)
	}

	// Re-process from the last output block, to the last accepted input block
	for inputBlock.Height() > outputBlock.Height() {
		reprocessBlk, err := v.chainIndex.GetBlockByHeight(ctx, outputBlock.Height()+1)
		if err != nil {
			return nil, err
		}
		outputBlock, err = v.chain.Execute(ctx, outputBlock, reprocessBlk)
		if err != nil {
			return nil, err
		}
		acceptedBlock, err = v.chain.AcceptBlock(ctx, outputBlock)
		if err != nil {
			return nil, err
		}
	}

	return NewAcceptedBlock(v.covariantVM, inputBlock, outputBlock, acceptedBlock), nil
}

func (v *VM[I, O, A]) setLastAccepted(lastAcceptedBlock *StatefulBlock[I, O, A]) {
	v.lastAcceptedBlock = lastAcceptedBlock
	v.preferredBlkID = v.lastAcceptedBlock.ID()
	v.acceptedBlocksByHeight.Put(v.lastAcceptedBlock.Height(), v.lastAcceptedBlock.ID())
	v.acceptedBlocksByID.Put(v.lastAcceptedBlock.ID(), v.lastAcceptedBlock)
}

func (v *VM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (snowman.Block, error) {
	return v.covariantVM.GetBlock(ctx, blkID)
}

func (v *VM[I, O, A]) GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlockIDAtHeight")
	defer span.End()

	if blkHeight == v.lastAcceptedBlock.Height() {
		return v.lastAcceptedBlock.ID(), nil
	}
	if blkID, ok := v.acceptedBlocksByHeight.Get(blkHeight); ok {
		return blkID, nil
	}
	return v.chainIndex.GetBlockIDAtHeight(ctx, blkHeight)
}

func (v *VM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (snowman.Block, error) {
	return v.covariantVM.ParseBlock(ctx, bytes)
}

func (v *VM[I, O, A]) BuildBlock(ctx context.Context) (snowman.Block, error) {
	return v.covariantVM.BuildBlock(ctx)
}

func (v *VM[I, O, A]) SetPreference(ctx context.Context, blkID ids.ID) error {
	v.preferredBlkID = blkID
	return nil
}

func (v *VM[I, O, A]) LastAccepted(context.Context) (ids.ID, error) {
	return v.lastAcceptedBlock.ID(), nil
}

func (v *VM[I, O, A]) SetState(ctx context.Context, state snow.State) error {
	switch state {
	case snow.StateSyncing:
		v.log.Info("Starting state sync")
		return nil
	case snow.Bootstrapping:
		v.log.Info("Starting bootstrapping")

		for _, startBootstrappingF := range v.Options.OnBootstrapStarted {
			if err := startBootstrappingF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.NormalOp:
		v.log.Info("Starting normal operation", zap.Bool("stateSyncStarted", v.Options.StateSyncClient.Started()))
		for _, startNormalOpF := range v.Options.OnNormalOperationStarted {
			if err := startNormalOpF(ctx); err != nil {
				return err
			}
		}
		return nil
	default:
		return snow.ErrUnknownState
	}
}

func (v *VM[I, O, A]) HealthCheck(ctx context.Context) (interface{}, error) {
	return v.Options.HealthChecker.HealthCheck(ctx)
}

func (v *VM[I, O, A]) CreateHandlers(ctx context.Context) (map[string]http.Handler, error) {
	return v.Options.Handlers, nil
}

func (v *VM[I, O, A]) Shutdown(context.Context) error {
	close(v.shutdownChan)

	errs := make([]error, len(v.Options.Closers))
	for i, closer := range v.Options.Closers {
		errs[i] = closer()
	}
	return errors.Join(errs...)
}

func (v *VM[I, O, A]) Version(context.Context) (string, error) {
	return v.Options.Version, nil
}
