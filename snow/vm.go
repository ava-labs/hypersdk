// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/utils"

	avacache "github.com/ava-labs/avalanchego/cache"
	hcontext "github.com/ava-labs/hypersdk/context"
)

var _ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)

type ChainInput struct {
	SnowCtx                    *snow.Context
	GenesisBytes, UpgradeBytes []byte
	ToEngine                   chan<- common.Message
	Shutdown                   <-chan struct{}
	Tracer                     trace.Tracer
	Config                     hcontext.Config
}

type MakeChainIndexFunc[I Block, O Block, A Block] func(
	ctx context.Context,
	chainIndex ChainIndex[I],
	outputBlock O,
	acceptedBlock A,
	stateReady bool,
) (*ConsensusIndex[I, O, A], error)

type Chain[I Block, O Block, A Block] interface {
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		app *Application[I, O, A],
	) (inputChainIndex ChainIndex[I], lastOutput O, lastAccepted A, stateReady bool, err error)
	// SetConsensusIndex sets the ChainIndex[I, O, A} on the VM to provide the
	// VM with:
	// 1. A cached index of the chain
	// 2. The ability to fetch the latest consensus state (preferred output block and last accepted block)
	SetConsensusIndex(*ConsensusIndex[I, O, A])
	BuildBlock(ctx context.Context, parent O) (I, O, error)
	ParseBlock(ctx context.Context, bytes []byte) (I, error)
	VerifyBlock(
		ctx context.Context,
		parent O,
		block I,
	) (O, error)
	AcceptBlock(ctx context.Context, acceptedParent A, outputBlock O) (A, error)
}

type VM[I Block, O Block, A Block] struct {
	chainLock       sync.Mutex
	chain           Chain[I, O, A]
	inputChainIndex ChainIndex[I]
	consensusIndex  *ConsensusIndex[I, O, A]
	app             Application[I, O, A]

	snowCtx  *snow.Context
	hconfig  hcontext.Config
	vmConfig VMConfig

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

	metaLock          sync.Mutex
	lastAcceptedBlock *StatefulBlock[I, O, A]
	preferredBlkID    ids.ID

	readyL sync.RWMutex
	ready  bool

	metrics *Metrics
	log     logging.Logger
	tracer  trace.Tracer

	shutdownChan chan struct{}
}

func NewVM[I Block, O Block, A Block](version string, chain Chain[I, O, A]) *VM[I, O, A] {
	v := &VM[I, O, A]{
		chain: chain,
		app: Application[I, O, A]{
			Version: version,
			HealthChecker: health.CheckerFunc(func(_ context.Context) (interface{}, error) {
				return nil, nil
			}),
			Handlers:     make(map[string]http.Handler),
			AcceptedSubs: make([]event.Subscription[A], 0),
		},
	}
	v.app.vm = v
	return v
}

func (v *VM[I, O, A]) Initialize(
	ctx context.Context,
	chainCtx *snow.Context,
	_ database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
	appSender common.AppSender,
) error {
	v.snowCtx = chainCtx
	v.shutdownChan = make(chan struct{})

	hconfig, err := hcontext.NewConfig(configBytes)
	if err != nil {
		return fmt.Errorf("failed to create hypersdk config: %w", err)
	}
	v.hconfig = hconfig
	tracerConfig, err := GetTracerConfig(hconfig)
	if err != nil {
		return fmt.Errorf("failed to fetch tracer config: %w", err)
	}
	tracer, err := trace.New(tracerConfig)
	if err != nil {
		return err
	}
	v.tracer = tracer
	ctx, span := v.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	v.vmConfig, err = GetVMConfig(hconfig)
	if err != nil {
		return fmt.Errorf("failed to parse vm config: %w", err)
	}

	defaultRegistry, err := metrics.MakeAndRegister(v.snowCtx.Metrics, "snow")
	if err != nil {
		return err
	}
	metrics, err := newMetrics(defaultRegistry)
	if err != nil {
		return err
	}
	v.metrics = metrics
	v.log = chainCtx.Log

	continuousProfilerConfig, err := GetProfilerConfig(hconfig)
	if err != nil {
		return fmt.Errorf("failed to parse continuous profiler config: %w", err)
	}
	if continuousProfilerConfig.Enabled {
		continuousProfiler := profiler.NewContinuous(
			continuousProfilerConfig.Dir,
			continuousProfilerConfig.Freq,
			continuousProfilerConfig.MaxNumFiles,
		)
		v.app.AddCloser("continuous profiler", func() error {
			continuousProfiler.Shutdown()
			return nil
		})
		go continuousProfiler.Dispatch() //nolint:errcheck
	}

	v.app.Network, err = p2p.NewNetwork(v.log, appSender, defaultRegistry, "p2p")
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
		ToEngine:     toEngine,
		Shutdown:     v.shutdownChan,
		Tracer:       v.tracer,
		Config:       v.hconfig,
	}

	inputChainIndex, lastOutput, lastAccepted, stateReady, err := v.chain.Initialize(
		ctx,
		chainInput,
		&v.app,
	)
	if err != nil {
		return err
	}
	v.inputChainIndex = inputChainIndex
	if err := v.makeChainIndex(ctx, v.inputChainIndex, lastOutput, lastAccepted, stateReady); err != nil {
		return err
	}
	v.chain.SetConsensusIndex(v.consensusIndex)
	if err := v.lastAcceptedBlock.notifyAccepted(ctx); err != nil {
		return fmt.Errorf("failed to notify last accepted on startup: %w", err)
	}
	return nil
}

func (v *VM[I, O, A]) setLastAccepted(lastAcceptedBlock *StatefulBlock[I, O, A]) {
	v.metaLock.Lock()
	defer v.metaLock.Unlock()

	v.lastAcceptedBlock = lastAcceptedBlock
	v.acceptedBlocksByHeight.Put(v.lastAcceptedBlock.Height(), v.lastAcceptedBlock.ID())
	v.acceptedBlocksByID.Put(v.lastAcceptedBlock.ID(), v.lastAcceptedBlock)
}

func (v *VM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlock")
	defer span.End()

	// Check verified map
	v.verifiedL.RLock()
	if blk, exists := v.verifiedBlocks[blkID]; exists {
		v.verifiedL.RUnlock()
		return blk, nil
	}
	v.verifiedL.RUnlock()

	// Check if last accepted block or recently accepted
	if v.lastAcceptedBlock.ID() == blkID {
		return v.lastAcceptedBlock, nil
	}
	if blk, ok := v.acceptedBlocksByID.Get(blkID); ok {
		return blk, nil
	}
	// Retrieve and parse from disk
	// Note: this returns an accepted block with only the input block set.
	// The consensus engine guarantees that:
	// 1. Verify is only called on a block whose parent is lastAcceptedBlock or in verifiedBlocks
	// 2. Accept is only called on a block whose parent is lastAcceptedBlock
	blk, err := v.inputChainIndex.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}
	return NewInputBlock(v, blk), nil
}

func (v *VM[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlockByHeight")
	defer span.End()

	if v.lastAcceptedBlock.Height() == height {
		return v.lastAcceptedBlock, nil
	}
	var blkID ids.ID
	if fetchedBlkID, ok := v.acceptedBlocksByHeight.Get(height); ok {
		blkID = fetchedBlkID
	} else {
		fetchedBlkID, err := v.inputChainIndex.GetBlockIDAtHeight(ctx, height)
		if err != nil {
			return nil, err
		}
		blkID = fetchedBlkID
	}

	if blk, ok := v.acceptedBlocksByID.Get(blkID); ok {
		return blk, nil
	}

	return v.GetBlock(ctx, blkID)
}

func (v *VM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.ParseBlock")
	defer span.End()

	start := time.Now()
	defer func() {
		v.metrics.blockParse.Observe(float64(time.Since(start)))
	}()

	blkID := utils.ToID(bytes)
	if existingBlk, err := v.GetBlock(ctx, blkID); err == nil {
		return existingBlk, nil
	}
	if blk, ok := v.parsedBlocks.Get(blkID); ok {
		return blk, nil
	}
	inputBlk, err := v.chain.ParseBlock(ctx, bytes)
	if err != nil {
		return nil, err
	}
	blk := NewInputBlock[I, O, A](v, inputBlk)
	v.parsedBlocks.Put(blkID, blk)
	return blk, nil
}

func (v *VM[I, O, A]) BuildBlock(ctx context.Context) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.BuildBlock")
	defer span.End()

	start := time.Now()
	defer func() {
		v.metrics.blockBuild.Observe(float64(time.Since(start)))
	}()

	preferredBlk, err := v.GetBlock(ctx, v.preferredBlkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferred block: %w", err)
	}
	inputBlock, outputBlock, err := v.chain.BuildBlock(ctx, preferredBlk.Output)
	if err != nil {
		return nil, err
	}
	sb := NewVerifiedBlock[I, O, A](v, inputBlock, outputBlock)
	v.parsedBlocks.Put(sb.ID(), sb)

	return sb, nil
}

// getExclusiveBlockRange returns the exclusive range of blocks (startBlock, endBlock)
func (v *VM[I, O, A]) getExclusiveBlockRange(ctx context.Context, startBlock *StatefulBlock[I, O, A], endBlock *StatefulBlock[I, O, A]) ([]*StatefulBlock[I, O, A], error) {
	if startBlock.ID() == endBlock.ID() {
		return nil, nil
	}

	diff, err := math.Sub(endBlock.Height(), startBlock.Height())
	if err != nil {
		return nil, fmt.Errorf("failed to calculate height difference for exclusive block range: %w", err)
	}
	if diff == 0 {
		return nil, fmt.Errorf("cannot fetch invalid block range (%s, %s)", startBlock, endBlock)
	}
	blkRange := make([]*StatefulBlock[I, O, A], 0, diff)
	blk := endBlock
	for {
		blk, err = v.GetBlock(ctx, blk.Parent())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch parent of %s while fetching exclusive block range (%s, %s): %w", blk, startBlock, endBlock, err)
		}
		if blk.ID() == startBlock.ID() {
			break
		}
		if blk.Height() <= startBlock.Height() {
			return nil, fmt.Errorf("invalid block range (%s, %s) terminated at %s", startBlock, endBlock, blk)
		}
		blkRange = append(blkRange, blk)
	}
	slices.Reverse(blkRange)
	return blkRange, nil
}

func (v *VM[I, O, A]) LastAcceptedBlock(_ context.Context) *StatefulBlock[I, O, A] {
	return v.lastAcceptedBlock
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
	return v.inputChainIndex.GetBlockIDAtHeight(ctx, blkHeight)
}

func (v *VM[I, O, A]) SetPreference(_ context.Context, blkID ids.ID) error {
	v.metaLock.Lock()
	defer v.metaLock.Unlock()

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

		for _, startStateSyncF := range v.app.OnStateSyncStarted {
			if err := startStateSyncF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.Bootstrapping:
		v.log.Info("Starting bootstrapping")

		for _, startBootstrappingF := range v.app.OnBootstrapStarted {
			if err := startBootstrappingF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.NormalOp:
		v.log.Info("Starting normal operation")
		for _, startNormalOpF := range v.app.OnNormalOperationStarted {
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
	return v.app.HealthChecker.HealthCheck(ctx)
}

func (v *VM[I, O, A]) CreateHandlers(_ context.Context) (map[string]http.Handler, error) {
	return v.app.Handlers, nil
}

func (v *VM[I, O, A]) Shutdown(context.Context) error {
	v.log.Info("Shutting down VM")
	close(v.shutdownChan)

	errs := make([]error, len(v.app.Closers))
	for i, closer := range v.app.Closers {
		v.log.Info("Shutting down service", zap.String("service", closer.name))
		start := time.Now()
		errs[i] = closer.close()
		v.log.Info("Finished shutting down service", zap.String("service", closer.name), zap.Duration("duration", time.Since(start)))
	}
	return errors.Join(errs...)
}

func (v *VM[I, O, A]) Version(context.Context) (string, error) {
	return v.app.Version, nil
}

func (v *VM[I, O, A]) isReady() bool {
	v.readyL.RLock()
	defer v.readyL.RUnlock()

	return v.ready
}

func (v *VM[I, O, A]) markReady(ready bool) {
	v.readyL.Lock()
	defer v.readyL.Unlock()

	v.ready = ready
}
