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

var _ block.StateSyncableVM = (*vm[Block, Block, Block])(nil)

type ChainInput struct {
	SnowCtx                    *snow.Context
	GenesisBytes, UpgradeBytes []byte
	ToEngine                   chan<- common.Message
	Shutdown                   <-chan struct{}
	Tracer                     trace.Tracer
	Config                     hcontext.Config
}

// Chain provides a reduced interface for chain / VM developers to implement.
// The snow package implements the full AvalancheGo VM interface, so that the chain implementation
// only needs to provide a way to initialize itself with all of the provided inputs, define concrete types
// for each state that a block could be in from the perspective of the consensus engine, and state
// transition functions between each of those types.
type Chain[I Block, O Block, A Block] interface {
	// Initialize initializes the chain, optionally configures the VM via app, and returns
	// a persistent index of the chain's input block type, the last output and accetped block,
	// and whether or not the VM is currently in a valid state. If stateReady is false, the VM
	// must be mid-state sync, such that it does not have a valid last output or accepted block.
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		vm VM[I, O, A],
	) (inputChainIndex ChainIndex[I], lastOutput O, lastAccepted A, stateReady bool, err error)
	// SetConsensusIndex sets the ChainIndex[I, O, A} on the VM to provide the
	// VM with:
	// 1. A cached index of the chain
	// 2. The ability to fetch the latest consensus state (preferred output block and last accepted block)
	SetConsensusIndex(consensusIndex *ConsensusIndex[I, O, A])
	// BuildBlock returns a new input and output block built on top of the provided parent.
	// The provided parent will be the current preference of the consensus engine.
	BuildBlock(ctx context.Context, parent O) (I, O, error)
	// ParseBlock parses the provided bytes into an input block.
	ParseBlock(ctx context.Context, bytes []byte) (I, error)
	// VerifyBlock verifies the provided block is valid given its already verified parent
	// and returns the resulting output of executing the block.
	VerifyBlock(
		ctx context.Context,
		parent O,
		block I,
	) (O, error)
	// AcceptBlock marks block as accepted and returns the resulting Accepted block type.
	// AcceptBlock is guaranteed to be called after the input block has been persisted
	// to disk.
	AcceptBlock(ctx context.Context, acceptedParent A, block O) (A, error)
}

type namedCloser struct {
	name  string
	close func() error
}

type vm[I Block, O Block, A Block] struct {
	handlers        map[string]http.Handler
	healthChecker   health.Checker
	network         *p2p.Network
	stateSyncableVM block.StateSyncableVM
	closers         []namedCloser

	onStateSyncStarted        []func(context.Context) error
	onBootstrapStarted        []func(context.Context) error
	onNormalOperationsStarted []func(context.Context) error

	verifiedSubs         []event.Subscription[O]
	rejectedSubs         []event.Subscription[O]
	acceptedSubs         []event.Subscription[A]
	preReadyAcceptedSubs []event.Subscription[I]
	version              string

	// chainLock must be held to process a block with chain (Build/Verify/Accept).
	chainLock       sync.Mutex
	chain           Chain[I, O, A]
	inputChainIndex ChainIndex[I]
	consensusIndex  *ConsensusIndex[I, O, A]

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

func newVM[I Block, O Block, A Block](version string, chain Chain[I, O, A]) *vm[I, O, A] {
	return &vm[I, O, A]{
		handlers: make(map[string]http.Handler),
		healthChecker: health.CheckerFunc(func(_ context.Context) (interface{}, error) {
			return nil, nil
		}),
		version: version,
		chain:   chain,
	}
}

func (v *vm[I, O, A]) Initialize(
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
		v.addCloser("continuous profiler", func() error {
			continuousProfiler.Shutdown()
			return nil
		})
		go continuousProfiler.Dispatch() //nolint:errcheck
	}

	v.network, err = p2p.NewNetwork(v.log, appSender, defaultRegistry, "p2p")
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
		VM[I, O, A]{vm: v},
	)
	if err != nil {
		return err
	}
	v.inputChainIndex = inputChainIndex
	if err := v.makeConsensusIndex(ctx, v.inputChainIndex, lastOutput, lastAccepted, stateReady); err != nil {
		return err
	}
	v.chain.SetConsensusIndex(v.consensusIndex)
	if err := v.lastAcceptedBlock.notifyAccepted(ctx); err != nil {
		return fmt.Errorf("failed to notify last accepted on startup: %w", err)
	}
	return nil
}

func (v *vm[I, O, A]) setLastAccepted(lastAcceptedBlock *StatefulBlock[I, O, A]) {
	v.metaLock.Lock()
	defer v.metaLock.Unlock()

	v.lastAcceptedBlock = lastAcceptedBlock
	v.acceptedBlocksByHeight.Put(v.lastAcceptedBlock.Height(), v.lastAcceptedBlock.ID())
	v.acceptedBlocksByID.Put(v.lastAcceptedBlock.ID(), v.lastAcceptedBlock)
}

func (v *vm[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (*StatefulBlock[I, O, A], error) {
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

func (v *vm[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (*StatefulBlock[I, O, A], error) {
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

func (v *vm[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (*StatefulBlock[I, O, A], error) {
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

func (v *vm[I, O, A]) BuildBlock(ctx context.Context) (*StatefulBlock[I, O, A], error) {
	v.chainLock.Lock()
	defer v.chainLock.Unlock()

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
func (v *vm[I, O, A]) getExclusiveBlockRange(ctx context.Context, startBlock *StatefulBlock[I, O, A], endBlock *StatefulBlock[I, O, A]) ([]*StatefulBlock[I, O, A], error) {
	if startBlock.ID() == endBlock.ID() {
		return nil, nil
	}

	diff, err := math.Sub(endBlock.Height(), startBlock.Height())
	if err != nil {
		return nil, fmt.Errorf("failed to calculate height difference for exclusive block range: %w", err)
	}
	// If the difference is 0, then the block range is invalid because we already checked they are not
	// the same block.
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

func (v *vm[I, O, A]) LastAcceptedBlock(_ context.Context) *StatefulBlock[I, O, A] {
	return v.lastAcceptedBlock
}

func (v *vm[I, O, A]) GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error) {
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

func (v *vm[I, O, A]) SetPreference(_ context.Context, blkID ids.ID) error {
	v.metaLock.Lock()
	defer v.metaLock.Unlock()

	v.preferredBlkID = blkID
	return nil
}

func (v *vm[I, O, A]) LastAccepted(context.Context) (ids.ID, error) {
	return v.lastAcceptedBlock.ID(), nil
}

func (v *vm[I, O, A]) SetState(ctx context.Context, state snow.State) error {
	switch state {
	case snow.StateSyncing:
		v.log.Info("Starting state sync")

		for _, startStateSyncF := range v.onStateSyncStarted {
			if err := startStateSyncF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.Bootstrapping:
		v.log.Info("Starting bootstrapping")

		for _, startBootstrappingF := range v.onBootstrapStarted {
			if err := startBootstrappingF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.NormalOp:
		v.log.Info("Starting normal operation")
		for _, startNormalOpF := range v.onNormalOperationsStarted {
			if err := startNormalOpF(ctx); err != nil {
				return err
			}
		}
		return nil
	default:
		return snow.ErrUnknownState
	}
}

func (v *vm[I, O, A]) HealthCheck(ctx context.Context) (interface{}, error) {
	return v.healthChecker.HealthCheck(ctx)
}

func (v *vm[I, O, A]) CreateHandlers(_ context.Context) (map[string]http.Handler, error) {
	return v.handlers, nil
}

func (v *vm[I, O, A]) Shutdown(context.Context) error {
	v.log.Info("Shutting down VM")
	close(v.shutdownChan)

	errs := make([]error, len(v.closers))
	for i, closer := range v.closers {
		v.log.Info("Shutting down service", zap.String("service", closer.name))
		start := time.Now()
		errs[i] = closer.close()
		v.log.Info("Finished shutting down service", zap.String("service", closer.name), zap.Duration("duration", time.Since(start)))
	}
	return errors.Join(errs...)
}

func (v *vm[I, O, A]) Version(context.Context) (string, error) {
	return v.version, nil
}

func (v *vm[I, O, A]) isReady() bool {
	v.readyL.RLock()
	defer v.readyL.RUnlock()

	return v.ready
}

func (v *vm[I, O, A]) markReady(ready bool) {
	v.readyL.Lock()
	defer v.readyL.Unlock()

	v.ready = ready
}

func (v *vm[I, O, A]) addCloser(name string, closer func() error) {
	v.closers = append(v.closers, namedCloser{name, closer})
}
