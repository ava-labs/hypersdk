// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

// Package snow provides a simplified interface ([snow.Chain]) for building Virtual Machines (VMs)
// on the Avalanche network by abstracting away the complexities of the consensus engine.
// It handles consensus integration and provides a slimmed-down interface for VM developers to implement.
//
// # Core Concepts
//
// Snow introduces a type-safe block state system using generics to represent the
// different states a block can be in during consensus:
//
//   - Block: The basic interface for blocks, requiring methods like GetID(), GetParent(), etc.
//   - Input (I): Unverified blocks that have been parsed but not validated
//   - Output (O): Verified blocks that have passed validation
//   - Accepted (A): Blocks that have been accepted by consensus and committed to the chain
//
// # Benefits
//
// By using the snow package, VM developers can:
//   - Focus on application-specific (VM) logic rather than consensus details
//   - Work with type-safe block representations at each stage
//   - Avoid common pitfalls in consensus integration
//   - Leverage built-in health checks, metrics, and state sync
package snow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/utils"

	avacache "github.com/ava-labs/avalanchego/cache"
	hcontext "github.com/ava-labs/hypersdk/context"
)

var _ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)

// ChainInput contains all external dependencies and configuration needed to
// initialize a Chain. It provides access to consensus context, network communication,
// initialization data (genesis/upgrade), tracing, and configuration.
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
type Chain[Input Block, Output Block, Accepted Block] interface {
	// Initialize initializes the chain, optionally configures the VM via app, and returns
	// a persistent index of the chain's input block type, the last output and accetped block,
	// and whether or not the VM is currently in a valid state. If stateReady is false, the VM
	// must be mid-state sync, such that it does not have a valid last output or accepted block.
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		vm *VM[Input, Output, Accepted],
	) (inputChainIndex ChainIndex[Input], lastOutput Output, lastAccepted Accepted, stateReady bool, err error)
	// SetConsensusIndex sets the ChainIndex[I, O, A} on the VM to provide the
	// VM with:
	// 1. A cached index of the chain
	// 2. The ability to fetch the latest consensus state (preferred output block and last accepted block)
	SetConsensusIndex(consensusIndex *ConsensusIndex[Input, Output, Accepted])
	// BuildBlock returns a new input and output block built on top of the provided parent.
	// The provided parent will be the current preference of the consensus engine.
	BuildBlock(ctx context.Context, blockContext *block.Context, parent Output) (Input, Output, error)
	// ParseBlock parses the provided bytes into an input block.
	ParseBlock(ctx context.Context, bytes []byte) (Input, error)
	// VerifyBlock verifies the provided block is valid given its already verified parent
	// and returns the resulting output of executing the block.
	VerifyBlock(
		ctx context.Context,
		parent Output,
		block Input,
	) (Output, error)
	// AcceptBlock marks block as accepted and returns the resulting Accepted block type.
	// AcceptBlock is guaranteed to be called after the input block has been persisted
	// to disk.
	AcceptBlock(ctx context.Context, acceptedParent Accepted, block Output) (Accepted, error)
}

type namedCloser struct {
	name  string
	close func() error
}

// VM implements the AvalancheGo common.VM interface by wrapping a custom Chain implementation.
// It manages the block lifecycle (parsing, verification, acceptance), handles consensus
// integration, maintains caching layers, and provides state synchronization by implementing block.StateSyncableVM.
// This design allows developers to focus on application logic while the VM handles
// consensus complexities.
type VM[Input Block, Output Block, Accepted Block] struct {
	handlers        map[string]http.Handler
	network         *p2p.Network
	stateSyncableVM block.StateSyncableVM
	closers         []namedCloser

	onStateSyncStarted        []func(context.Context) error
	onBootstrapStarted        []func(context.Context) error
	onNormalOperationsStarted []func(context.Context) error

	verifiedSubs         []event.Subscription[Output]
	rejectedSubs         []event.Subscription[Output]
	acceptedSubs         []event.Subscription[Accepted]
	preReadyAcceptedSubs []event.Subscription[Input]
	// preRejectedSubs handles rejections of I (Input) during/after state sync, before they reach O (Output) state
	preRejectedSubs []event.Subscription[Input]
	version         string

	// chainLock provides a synchronization point between state sync and normal operation.
	// To complete dynamic state sync, we must:
	// 1. Accept a sequence of blocks from the final state sync target to the last accepted block
	// 2. Re-process all outstanding processing blocks
	// 3. Mark the VM as ready for normal operation
	//
	// During this time, we must not allow any new blocks to be verified/accepted.
	chainLock sync.Mutex
	chain     Chain[Input, Output, Accepted]
	ready     bool

	inputChainIndex ChainIndex[Input]
	consensusIndex  *ConsensusIndex[Input, Output, Accepted]

	snowCtx  *snow.Context
	hconfig  hcontext.Config
	vmConfig VMConfig

	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *avacache.LRU[ids.ID, *StatefulBlock[Input, Output, Accepted]]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL sync.RWMutex

	// verifiedBlocks maintains a map of blocks that have passed verification but haven't yet been
	// accepted or rejected by consensus. It serves multiple critical purposes:
	// 1. Caches verified blocks
	// 2. Tracks blocks vacuously verified during dynamic state sync that need verifyProcessingBlocks after FinishStateSync
	// The map is protected by verifiedL to ensure thread-safety
	verifiedBlocks map[ids.ID]*StatefulBlock[Input, Output, Accepted]

	// We store the last [AcceptedBlockWindowCache] blocks in memory
	// to avoid reading blocks from disk.
	acceptedBlocksByID     *cache.FIFO[ids.ID, *StatefulBlock[Input, Output, Accepted]]
	acceptedBlocksByHeight *cache.FIFO[uint64, ids.ID]

	metaLock          sync.Mutex
	lastAcceptedBlock *StatefulBlock[Input, Output, Accepted]
	preferredBlkID    ids.ID

	metrics *Metrics
	log     logging.Logger
	tracer  trace.Tracer

	shutdownChan chan struct{}

	healthCheckers sync.Map
}

// NewVM creates a VM adapter that wraps a custom Chain implementation.
// This allows blockchain developers to focus on application-specific logic
// through the Chain interface while the snow package handles all
// consensus engine integration and complexity.
func NewVM[I Block, O Block, A Block](version string, chain Chain[I, O, A]) *VM[I, O, A] {
	return &VM[I, O, A]{
		handlers: make(map[string]http.Handler),
		version:  version,
		chain:    chain,
	}
}

// Initialize initializes the chain, optionally configures the VM via app, and returns
// a persistent index of the chain's input block type, the last output and accetped block,
// and whether or not the VM is currently in a valid state. If stateReady is false, the VM
// must be mid-state sync, such that it does not have a valid last output or accepted block.
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
		v,
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

	if err := v.initHealthCheckers(); err != nil {
		return err
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

// GetBlock retrieves a block by its ID following a specific lookup hierarchy:
// 1. First checks the verified blocks map
// 2. Then looks in the accepted blocks cache for recently accepted blocks
// 3. Finally falls back to retrieving from persistent storage if needed
// The returned StatefulBlock will have the appropriate state fields populated
// based on where it was found in the hierarchy.
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

// GetBlockByHeight attempts to retrieve an accepted block by height from the accepted blocks cache.
// Fallbacks to GetBlock
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

// ParseBlock parses a block from bytes and returns a NewInputBlock
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

// BuildBlockWithContext attempts to BuildBlock with block.Context
func (v *VM[I, O, A]) BuildBlockWithContext(ctx context.Context, blockCtx *block.Context) (*StatefulBlock[I, O, A], error) {
	return v.buildBlock(ctx, blockCtx)
}

// BuildBlock constructs a new block on top of the current preferred tip of the chain.
// It assembles and executes transactions from the mempool against the parent state,
// creating a fully verified StatefulBlock ready for consensus. Verification happens
// during construction, making the block immediately proposable regardless of the
// node's role as validator or producer.
func (v *VM[I, O, A]) BuildBlock(ctx context.Context) (*StatefulBlock[I, O, A], error) {
	return v.buildBlock(ctx, nil)
}

func (v *VM[I, O, A]) buildBlock(ctx context.Context, blockCtx *block.Context) (*StatefulBlock[I, O, A], error) {
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
		return nil, fmt.Errorf("failed to get preferred block %s to build: %w", v.preferredBlkID, err)
	}
	inputBlock, outputBlock, err := v.chain.BuildBlock(ctx, blockCtx, preferredBlk.Output)
	if err != nil {
		return nil, err
	}
	sb := NewVerifiedBlock[I, O, A](v, inputBlock, outputBlock)
	v.parsedBlocks.Put(sb.ID(), sb)

	return sb, nil
}

// LastAcceptedBlock returns the last accepted block
func (v *VM[I, O, A]) LastAcceptedBlock(_ context.Context) *StatefulBlock[I, O, A] {
	return v.lastAcceptedBlock
}

// GetBlockIDAtHeight returns the ID of the block at the given height
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

// SetPreference updates the VM's preferred block ID based on the consensus engine's decision.
func (v *VM[I, O, A]) SetPreference(_ context.Context, blkID ids.ID) error {
	v.metaLock.Lock()
	defer v.metaLock.Unlock()

	v.preferredBlkID = blkID
	return nil
}

// LastAccepted returns ID of the last accepted block
func (v *VM[I, O, A]) LastAccepted(context.Context) (ids.ID, error) {
	return v.lastAcceptedBlock.ID(), nil
}

// SetState sets the state of the VM
func (v *VM[I, O, A]) SetState(ctx context.Context, state snow.State) error {
	v.log.Info("Setting state to %s", zap.Stringer("state", state))
	switch state {
	case snow.StateSyncing:
		for _, startStateSyncF := range v.onStateSyncStarted {
			if err := startStateSyncF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.Bootstrapping:
		for _, startBootstrappingF := range v.onBootstrapStarted {
			if err := startBootstrappingF(ctx); err != nil {
				return err
			}
		}
		return nil
	case snow.NormalOp:
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

// CreateHandlers returns a map of HTTP handlers for the VM that can be used to interact with it over HTTP.
// CreateHandlers is a core implementation of common.VM
func (v *VM[I, O, A]) CreateHandlers(_ context.Context) (map[string]http.Handler, error) {
	return v.handlers, nil
}

// Shutdown shuts down the VM
func (v *VM[I, O, A]) Shutdown(context.Context) error {
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

// Version returns the version of the VM
func (v *VM[I, O, A]) Version(context.Context) (string, error) {
	return v.version, nil
}

func (v *VM[I, O, A]) addCloser(name string, closer func() error) {
	v.closers = append(v.closers, namedCloser{name, closer})
}

// GetInputCovariantVM returns *InputCovariantVM[I, O, A]
func (v *VM[I, O, A]) GetInputCovariantVM() *InputCovariantVM[I, O, A] {
	return &InputCovariantVM[I, O, A]{vm: v}
}

// GetNetwork returns VM's peer to peer network
func (v *VM[I, O, A]) GetNetwork() *p2p.Network {
	return v.network
}

// AddAcceptedSub adds subscriptions tracking accepted blocks
func (v *VM[I, O, A]) AddAcceptedSub(sub ...event.Subscription[A]) {
	v.acceptedSubs = append(v.acceptedSubs, sub...)
}

// AddRejectedSub adds subscriptions tracking rejected blocks
func (v *VM[I, O, A]) AddRejectedSub(sub ...event.Subscription[O]) {
	v.rejectedSubs = append(v.rejectedSubs, sub...)
}

// AddVerifiedSub adds subscriptions tracking verified blocks
func (v *VM[I, O, A]) AddVerifiedSub(sub ...event.Subscription[O]) {
	v.verifiedSubs = append(v.verifiedSubs, sub...)
}

// AddPreReadyAcceptedSub adds subscriptions tracking accepted blocks during state sync
func (v *VM[I, O, A]) AddPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	v.preReadyAcceptedSubs = append(v.preReadyAcceptedSubs, sub...)
}

// AddPreRejectedSub registers subscriptions that are notified when blocks are rejected
// after state sync.
//
// These subscriptions track blocks that were vacuously verified during state sync but later
// determined to be invalid or rejected by consensus
func (v *VM[I, O, A]) AddPreRejectedSub(sub ...event.Subscription[I]) {
	v.preRejectedSubs = append(v.preRejectedSubs, sub...)
}

// AddHandler adds a handler to the VM's http server.
func (v *VM[I, O, A]) AddHandler(name string, handler http.Handler) {
	v.handlers[name] = handler
}

// AddCloser adds a function called when the VM is shutting down
func (v *VM[I, O, A]) AddCloser(name string, closer func() error) {
	v.addCloser(name, closer)
}

// AddStateSyncStarter adds a function called when the VM transitions to the state sync state
func (v *VM[I, O, A]) AddStateSyncStarter(onStateSyncStarted ...func(context.Context) error) {
	v.onStateSyncStarted = append(v.onStateSyncStarted, onStateSyncStarted...)
}

// AddBootstrapStarter adds a function called when the VM transitions to the bootstrap state
func (v *VM[I, O, A]) AddBootstrapStarter(onBootstrapStartedF ...func(context.Context) error) {
	v.onBootstrapStarted = append(v.onBootstrapStarted, onBootstrapStartedF...)
}

// AddNormalOpStarter adds a function called when the VM transitions to the normal operation state
func (v *VM[I, O, A]) AddNormalOpStarter(onNormalOpStartedF ...func(context.Context) error) {
	v.onNormalOperationsStarted = append(v.onNormalOperationsStarted, onNormalOpStartedF...)
}
