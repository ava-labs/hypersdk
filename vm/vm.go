// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/network"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/workers"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
	avacache "github.com/ava-labs/avalanchego/cache"
	avatrace "github.com/ava-labs/avalanchego/trace"
	avautils "github.com/ava-labs/avalanchego/utils"
	avasync "github.com/ava-labs/avalanchego/x/sync"
)

type VM struct {
	c Controller
	v *version.Semantic

	snowCtx         *snow.Context
	pkBytes         []byte
	proposerMonitor *ProposerMonitor
	baseDB          database.Database

	config         Config
	genesis        Genesis
	builder        builder.Builder
	gossiper       gossiper.Gossiper
	rawStateDB     database.Database
	stateDB        merkledb.MerkleDB
	vmDB           database.Database
	handlers       Handlers
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry
	authEngine     map[uint8]AuthEngine

	tracer  avatrace.Tracer
	mempool *mempool.Mempool[*chain.Transaction]

	// track all accepted but still valid txs (replay protection)
	seen                   *emap.EMap[*chain.Transaction]
	startSeenTime          int64
	seenValidityWindowOnce sync.Once
	seenValidityWindow     chan struct{}

	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *avacache.LRU[ids.ID, *chain.StatelessBlock]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL      sync.RWMutex
	verifiedBlocks map[ids.ID]*chain.StatelessBlock

	// We store the last [AcceptedBlockWindowCache] blocks in memory
	// to avoid reading blocks from disk.
	acceptedBlocksByID     *cache.FIFO[ids.ID, *chain.StatelessBlock]
	acceptedBlocksByHeight *cache.FIFO[uint64, ids.ID]

	// Accepted block queue
	acceptedQueue chan *chain.StatelessBlock
	acceptorDone  chan struct{}

	// Transactions that streaming users are currently subscribed to
	webSocketServer *rpc.WebSocketServer

	// authVerifiers are used to verify signatures in parallel
	// with limited parallelism
	authVerifiers workers.Workers

	bootstrapped avautils.Atomic[bool]
	genesisBlk   *chain.StatelessBlock
	preferred    ids.ID
	lastAccepted *chain.StatelessBlock
	toEngine     chan<- common.Message

	// State Sync client and AppRequest handlers
	stateSyncClient        *stateSyncerClient
	stateSyncNetworkClient avasync.NetworkClient
	stateSyncNetworkServer *avasync.NetworkServer

	// Network manager routes p2p messages to pre-registered handlers
	networkManager *network.Manager

	metrics  *Metrics
	profiler profiler.ContinuousProfiler

	ready chan struct{}
	stop  chan struct{}
}

func New(c Controller, v *version.Semantic) *VM {
	return &VM{c: c, v: v}
}

// implements "block.ChainVM.common.VM"
func (vm *VM) Initialize(
	ctx context.Context,
	snowCtx *snow.Context,
	baseDB database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
	appSender common.AppSender,
) error {
	vm.snowCtx = snowCtx
	vm.pkBytes = bls.PublicKeyToBytes(vm.snowCtx.PublicKey)
	// This will be overwritten when we accept the first block (in state sync) or
	// backfill existing blocks (during normal bootstrapping).
	vm.startSeenTime = -1
	// Init seen for tracking transactions that have been accepted on-chain
	vm.seen = emap.NewEMap[*chain.Transaction]()
	vm.seenValidityWindow = make(chan struct{})
	vm.ready = make(chan struct{})
	vm.stop = make(chan struct{})
	gatherer := avametrics.NewMultiGatherer()
	if err := vm.snowCtx.Metrics.Register(gatherer); err != nil {
		return err
	}
	defaultRegistry, metrics, err := newMetrics()
	if err != nil {
		return err
	}
	if err := gatherer.Register("hypersdk", defaultRegistry); err != nil {
		return err
	}
	vm.metrics = metrics
	vm.proposerMonitor = NewProposerMonitor(vm)
	vm.networkManager = network.NewManager(vm.snowCtx.Log, vm.snowCtx.NodeID, appSender)

	vm.baseDB = baseDB

	// Always initialize implementation first
	vm.config, vm.genesis, vm.builder, vm.gossiper, vm.vmDB,
		vm.rawStateDB, vm.handlers, vm.actionRegistry, vm.authRegistry, vm.authEngine, err = vm.c.Initialize(
		vm,
		snowCtx,
		gatherer,
		genesisBytes,
		upgradeBytes,
		configBytes,
	)
	if err != nil {
		return fmt.Errorf("implementation initialization failed: %w", err)
	}

	// Setup tracer
	vm.tracer, err = trace.New(vm.config.GetTraceConfig())
	if err != nil {
		return err
	}
	ctx, span := vm.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	// Setup profiler
	if cfg := vm.config.GetContinuousProfilerConfig(); cfg.Enabled {
		vm.profiler = profiler.NewContinuous(cfg.Dir, cfg.Freq, cfg.MaxNumFiles)
		go vm.profiler.Dispatch() //nolint:errcheck
	}

	// Instantiate DBs
	merkleRegistry := prometheus.NewRegistry()
	vm.stateDB, err = merkledb.New(ctx, vm.rawStateDB, merkledb.Config{
		BranchFactor: vm.genesis.GetStateBranchFactor(),
		// RootGenConcurrency limits the number of goroutines
		// that will be used across all concurrent root generations.
		RootGenConcurrency:          uint(vm.config.GetRootGenerationCores()),
		HistoryLength:               uint(vm.config.GetStateHistoryLength()),
		ValueNodeCacheSize:          uint(vm.config.GetValueNodeCacheSize()),
		IntermediateNodeCacheSize:   uint(vm.config.GetIntermediateNodeCacheSize()),
		IntermediateWriteBufferSize: uint(vm.config.GetStateIntermediateWriteBufferSize()),
		IntermediateWriteBatchSize:  uint(vm.config.GetStateIntermediateWriteBatchSize()),
		Reg:                         merkleRegistry,
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.tracer,
	})
	if err != nil {
		return err
	}
	if err := gatherer.Register("state", merkleRegistry); err != nil {
		return err
	}

	// Setup worker cluster for verifying signatures
	//
	// If [parallelism] is odd, we assign the extra
	// core to signature verification.
	vm.authVerifiers = workers.NewParallel(vm.config.GetAuthVerificationCores(), 100) // TODO: make job backlog a const

	// Init channels before initializing other structs
	vm.toEngine = toEngine

	vm.parsedBlocks = &avacache.LRU[ids.ID, *chain.StatelessBlock]{Size: vm.config.GetParsedBlockCacheSize()}
	vm.verifiedBlocks = make(map[ids.ID]*chain.StatelessBlock)
	vm.acceptedBlocksByID, err = cache.NewFIFO[ids.ID, *chain.StatelessBlock](vm.config.GetAcceptedBlockWindowCache())
	if err != nil {
		return err
	}
	vm.acceptedBlocksByHeight, err = cache.NewFIFO[uint64, ids.ID](vm.config.GetAcceptedBlockWindowCache())
	if err != nil {
		return err
	}
	vm.acceptedQueue = make(chan *chain.StatelessBlock, vm.config.GetAcceptorSize())
	vm.acceptorDone = make(chan struct{})

	vm.mempool = mempool.New[*chain.Transaction](
		vm.tracer,
		vm.config.GetMempoolSize(),
		vm.config.GetMempoolSponsorSize(),
		vm.config.GetMempoolExemptSponsors(),
	)

	// Try to load last accepted
	has, err := vm.HasLastAccepted()
	if err != nil {
		snowCtx.Log.Error("could not determine if have last accepted")
		return err
	}
	if has { //nolint:nestif
		genesisBlk, err := vm.GetGenesis(ctx)
		if err != nil {
			snowCtx.Log.Error("could not get genesis", zap.Error(err))
			return err
		}
		vm.genesisBlk = genesisBlk
		lastAcceptedHeight, err := vm.GetLastAcceptedHeight()
		if err != nil {
			snowCtx.Log.Error("could not get last accepted height", zap.Error(err))
			return err
		}
		blk, err := vm.GetDiskBlock(ctx, lastAcceptedHeight)
		if err != nil {
			snowCtx.Log.Error("could not get last accepted block", zap.Error(err))
			return err
		}
		vm.preferred, vm.lastAccepted = blk.ID(), blk
		if err := vm.loadAcceptedBlocks(ctx); err != nil {
			snowCtx.Log.Error("could not load accepted blocks from disk", zap.Error(err))
			return err
		}
		// It is not guaranteed that the last accepted state on-disk matches the post-execution
		// result of the last accepted block.
		snowCtx.Log.Info("initialized vm from last accepted", zap.Stringer("block", blk.ID()))
	} else {
		// Set balances and compute genesis root
		sps := state.NewSimpleMutable(vm.stateDB)
		if err := vm.genesis.Load(ctx, vm.tracer, sps); err != nil {
			snowCtx.Log.Error("could not set genesis allocation", zap.Error(err))
			return err
		}
		if err := sps.Commit(ctx); err != nil {
			return err
		}
		root, err := vm.stateDB.GetMerkleRoot(ctx)
		if err != nil {
			snowCtx.Log.Error("could not get merkle root", zap.Error(err))
			return err
		}
		snowCtx.Log.Info("genesis state created", zap.Stringer("root", root))

		// Create genesis block
		genesisBlk, err := chain.ParseStatefulBlock(
			ctx,
			chain.NewGenesisBlock(root),
			nil,
			choices.Accepted,
			vm,
		)
		if err != nil {
			snowCtx.Log.Error("unable to init genesis block", zap.Error(err))
			return err
		}

		// Update chain metadata
		sps = state.NewSimpleMutable(vm.stateDB)
		if err := sps.Insert(ctx, chain.HeightKey(vm.StateManager().HeightKey()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
			return err
		}
		if err := sps.Insert(ctx, chain.TimestampKey(vm.StateManager().TimestampKey()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
			return err
		}
		genesisRules := vm.c.Rules(0)
		feeManager := fees.NewManager(nil)
		minUnitPrice := genesisRules.GetMinUnitPrice()
		for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
			feeManager.SetUnitPrice(i, minUnitPrice[i])
			snowCtx.Log.Info("set genesis unit price", zap.Int("dimension", int(i)), zap.Uint64("price", feeManager.UnitPrice(i)))
		}
		if err := sps.Insert(ctx, chain.FeeKey(vm.StateManager().FeeKey()), feeManager.Bytes()); err != nil {
			return err
		}

		// Commit genesis block post-execution state and compute root
		if err := sps.Commit(ctx); err != nil {
			return err
		}
		genesisRoot, err := vm.stateDB.GetMerkleRoot(ctx)
		if err != nil {
			snowCtx.Log.Error("could not get merkle root", zap.Error(err))
			return err
		}

		// Update last accepted and preferred block
		vm.genesisBlk = genesisBlk
		if err := vm.UpdateLastAccepted(genesisBlk); err != nil {
			snowCtx.Log.Error("could not set genesis block as last accepted", zap.Error(err))
			return err
		}
		gBlkID := genesisBlk.ID()
		vm.preferred, vm.lastAccepted = gBlkID, genesisBlk
		snowCtx.Log.Info("initialized vm from genesis",
			zap.Stringer("block", gBlkID),
			zap.Stringer("pre-execution root", genesisBlk.StateRoot),
			zap.Stringer("post-execution root", genesisRoot),
		)
	}
	go vm.processAcceptedBlocks()

	// Setup state syncing
	stateSyncHandler, stateSyncSender := vm.networkManager.Register()
	syncRegistry := prometheus.NewRegistry()
	vm.stateSyncNetworkClient, err = avasync.NewNetworkClient(
		stateSyncSender,
		vm.snowCtx.NodeID,
		int64(vm.config.GetStateSyncParallelism()),
		vm.Logger(),
		"",
		syncRegistry,
	)
	if err != nil {
		return err
	}
	if err := gatherer.Register("sync", syncRegistry); err != nil {
		return err
	}
	vm.stateSyncClient = vm.NewStateSyncClient(gatherer)
	vm.stateSyncNetworkServer = avasync.NewNetworkServer(stateSyncSender, vm.stateDB, vm.Logger())
	vm.networkManager.SetHandler(stateSyncHandler, NewStateSyncHandler(vm))

	// Setup gossip networking
	gossipHandler, gossipSender := vm.networkManager.Register()
	vm.networkManager.SetHandler(gossipHandler, NewTxGossipHandler(vm))

	// Startup block builder and gossiper
	go vm.builder.Run()
	go vm.gossiper.Run(gossipSender)

	// Wait until VM is ready and then send a state sync message to engine
	go vm.markReady()

	// Setup handlers
	jsonRPCHandler, err := rpc.NewJSONRPCHandler(rpc.Name, rpc.NewJSONRPCServer(vm))
	if err != nil {
		return fmt.Errorf("unable to create handler: %w", err)
	}
	if _, ok := vm.handlers[rpc.JSONRPCEndpoint]; ok {
		return fmt.Errorf("duplicate JSONRPC handler found: %s", rpc.JSONRPCEndpoint)
	}
	vm.handlers[rpc.JSONRPCEndpoint] = jsonRPCHandler
	if _, ok := vm.handlers[rpc.WebSocketEndpoint]; ok {
		return fmt.Errorf("duplicate WebSocket handler found: %s", rpc.WebSocketEndpoint)
	}
	webSocketServer, pubsubServer := rpc.NewWebSocketServer(vm, vm.config.GetStreamingBacklogSize())
	vm.webSocketServer = webSocketServer
	vm.handlers[rpc.WebSocketEndpoint] = pubsubServer
	return nil
}

func (vm *VM) checkActivity(ctx context.Context) {
	vm.gossiper.Queue(ctx)
	vm.builder.Queue(ctx)
}

func (vm *VM) markReady() {
	// Wait for state syncing to complete
	select {
	case <-vm.stop:
		return
	case <-vm.stateSyncClient.done:
	}

	// We can begin partailly verifying blocks here because
	// we have the full state but can't detect duplicate transactions
	// because we haven't yet observed a full [ValidityWindow].
	vm.snowCtx.Log.Info("state sync client ready")

	// Wait for a full [ValidityWindow] before
	// we are willing to vote on blocks.
	select {
	case <-vm.stop:
		return
	case <-vm.seenValidityWindow:
	}
	vm.snowCtx.Log.Info("validity window ready")
	if vm.stateSyncClient.Started() {
		vm.toEngine <- common.StateSyncDone
	}
	close(vm.ready)

	// Mark node ready and attempt to build a block.
	vm.snowCtx.Log.Info(
		"node is now ready",
		zap.Bool("synced", vm.stateSyncClient.Started()),
	)
	vm.checkActivity(context.TODO())
}

func (vm *VM) isReady() bool {
	select {
	case <-vm.ready:
		return true
	default:
		vm.snowCtx.Log.Info("node is not ready yet")
		return false
	}
}

// TODO: remove?
func (vm *VM) BaseDB() database.Database {
	return vm.baseDB
}

func (vm *VM) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	if !vm.isReady() {
		return utils.Repeat[[]byte](nil, len(keys)), utils.Repeat(ErrNotReady, len(keys))
	}
	// Atomic read to ensure consistency
	return vm.stateDB.GetValues(ctx, keys)
}

func (vm *VM) SetState(_ context.Context, state snow.State) error {
	switch state {
	case snow.StateSyncing:
		vm.Logger().Info("state sync started")
		return nil
	case snow.Bootstrapping:
		// Ensure state sync client marks itself as done if it was never started
		syncStarted := vm.stateSyncClient.Started()
		if !syncStarted {
			// We must check if we finished syncing before starting bootstrapping.
			// This should only ever occur if we began a state sync, restarted, and
			// were unable to find any acceptable summaries.
			syncing, err := vm.GetDiskIsSyncing()
			if err != nil {
				vm.Logger().Error("could not determine if syncing", zap.Error(err))
				return err
			}
			if syncing {
				vm.Logger().Error("cannot start bootstrapping", zap.Error(ErrStateSyncing))
				// This is a fatal error that will require retrying sync or deleting the
				// node database.
				return ErrStateSyncing
			}

			// If we weren't previously syncing, we force state syncer completion so
			// that the node will mark itself as ready.
			vm.stateSyncClient.ForceDone()

			// TODO: add a config to FATAL here if could not state sync (likely won't be
			// able to recover in networks where no one has the full state, bypass
			// still starts sync): https://github.com/ava-labs/hypersdk/issues/438
		}

		// Backfill seen transactions, if any. This will exit as soon as we reach
		// a block we no longer have on disk or if we have walked back the full
		// [ValidityWindow].
		vm.backfillSeenTransactions()

		// Trigger that bootstrapping has started
		vm.Logger().Info("bootstrapping started", zap.Bool("state sync started", syncStarted))
		return vm.onBootstrapStarted()
	case snow.NormalOp:
		vm.Logger().
			Info("normal operation started", zap.Bool("state sync started", vm.stateSyncClient.Started()))
		return vm.onNormalOperationsStarted()
	default:
		return snow.ErrUnknownState
	}
}

// onBootstrapStarted marks this VM as bootstrapping
func (vm *VM) onBootstrapStarted() error {
	vm.bootstrapped.Set(false)
	return nil
}

// ForceReady is used in integration testing
func (vm *VM) ForceReady() {
	// Only works if haven't already started syncing
	vm.stateSyncClient.ForceDone()
	vm.seenValidityWindowOnce.Do(func() {
		close(vm.seenValidityWindow)
	})
}

// onNormalOperationsStarted marks this VM as bootstrapped
func (vm *VM) onNormalOperationsStarted() error {
	defer vm.checkActivity(context.TODO())

	if vm.bootstrapped.Get() {
		return nil
	}
	vm.bootstrapped.Set(true)
	return nil
}

// implements "block.ChainVM.common.VM"
func (vm *VM) Shutdown(ctx context.Context) error {
	close(vm.stop)

	// Shutdown state sync client if still running
	if err := vm.stateSyncClient.Shutdown(); err != nil {
		return err
	}

	// Process remaining accepted blocks before shutdown
	close(vm.acceptedQueue)
	<-vm.acceptorDone

	// Shutdown other async VM mechanisms
	vm.builder.Done()
	vm.gossiper.Done()
	vm.authVerifiers.Stop()
	if vm.profiler != nil {
		vm.profiler.Shutdown()
	}

	// Shutdown controller once all mechanisms that could invoke it have
	// shutdown.
	if err := vm.c.Shutdown(ctx); err != nil {
		return err
	}

	// Close DBs
	if vm.snowCtx == nil {
		return nil
	}
	if err := vm.vmDB.Close(); err != nil {
		return err
	}
	if err := vm.stateDB.Close(); err != nil {
		return err
	}
	return vm.rawStateDB.Close()
}

// implements "block.ChainVM.common.VM"
// TODO: this must be callable in the factory before initializing
func (vm *VM) Version(_ context.Context) (string, error) { return vm.v.String(), nil }

// implements "block.ChainVM.common.VM"
// for "ext/vm/[chainID]"
func (vm *VM) CreateHandlers(_ context.Context) (map[string]http.Handler, error) {
	return vm.handlers, nil
}

// implements "block.ChainVM.commom.VM.health.Checkable"
func (vm *VM) HealthCheck(context.Context) (interface{}, error) {
	// TODO: engine will mark VM as ready when we return
	// [block.StateSyncDynamic]. This should change in v1.9.11.
	//
	// We return "unhealthy" here until synced to block RPC traffic in the
	// meantime.
	if !vm.isReady() {
		return http.StatusServiceUnavailable, ErrNotReady
	}
	return http.StatusOK, nil
}

// implements "block.ChainVM.commom.VM.Getter"
// replaces "core.SnowmanVM.GetBlock"
//
// This is ONLY called on accepted blocks pre-ProposerVM fork.
func (vm *VM) GetBlock(ctx context.Context, id ids.ID) (snowman.Block, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.GetBlock")
	defer span.End()

	// We purposely don't return parsed but unverified blocks from here
	return vm.GetStatelessBlock(ctx, id)
}

func (vm *VM) GetStatelessBlock(ctx context.Context, blkID ids.ID) (*chain.StatelessBlock, error) {
	_, span := vm.tracer.Start(ctx, "VM.GetStatelessBlock")
	defer span.End()

	// Check if verified block
	vm.verifiedL.RLock()
	if blk, exists := vm.verifiedBlocks[blkID]; exists {
		vm.verifiedL.RUnlock()
		return blk, nil
	}
	vm.verifiedL.RUnlock()

	// Check if last accepted
	if vm.lastAccepted.ID() == blkID {
		return vm.lastAccepted, nil
	}

	// Check if genesis
	if vm.genesisBlk.ID() == blkID {
		return vm.genesisBlk, nil
	}

	// Check if recently accepted block
	if blk, ok := vm.acceptedBlocksByID.Get(blkID); ok {
		return blk, nil
	}

	// Check to see if the block is on disk
	blkHeight, err := vm.GetBlockIDHeight(blkID)
	if err != nil {
		return nil, err
	}
	// We wait to count this metric until we know we have
	// the index on-disk because peers may query us for
	// blocks we don't have yet at tip and we don't want
	// to count that as a historical read.
	vm.metrics.blocksFromDisk.Inc()
	return vm.GetDiskBlock(ctx, blkHeight)
}

// implements "block.ChainVM.commom.VM.Parser"
// replaces "core.SnowmanVM.ParseBlock"
func (vm *VM) ParseBlock(ctx context.Context, source []byte) (snowman.Block, error) {
	start := time.Now()
	defer func() {
		vm.metrics.blockParse.Observe(float64(time.Since(start)))
	}()

	ctx, span := vm.tracer.Start(ctx, "VM.ParseBlock")
	defer span.End()

	// Check to see if we've already parsed
	id := utils.ToID(source)

	// If we have seen this block before, return it with the most
	// up-to-date info
	if oldBlk, err := vm.GetStatelessBlock(ctx, id); err == nil {
		vm.snowCtx.Log.Debug("returning previously parsed block", zap.Stringer("id", oldBlk.ID()))
		return oldBlk, nil
	}

	// Attempt to parse and cache block
	blk, exist := vm.parsedBlocks.Get(id)
	if exist {
		return blk, nil
	}
	newBlk, err := chain.ParseBlock(
		ctx,
		source,
		choices.Processing,
		vm,
	)
	if err != nil {
		vm.snowCtx.Log.Error("could not parse block", zap.Stringer("blkID", id), zap.Error(err))
		return nil, err
	}
	vm.parsedBlocks.Put(id, newBlk)
	vm.snowCtx.Log.Info(
		"parsed block",
		zap.Stringer("id", newBlk.ID()),
		zap.Uint64("height", newBlk.Hght),
	)
	return newBlk, nil
}

// implements "block.ChainVM"
func (vm *VM) BuildBlock(ctx context.Context) (snowman.Block, error) {
	start := time.Now()
	defer func() {
		vm.metrics.blockBuild.Observe(float64(time.Since(start)))
	}()

	ctx, span := vm.tracer.Start(ctx, "VM.BuildBlock")
	defer span.End()

	// If the node isn't ready, we should exit.
	//
	// We call [QueueNotify] when the VM becomes ready, so exiting
	// early here should not cause us to stop producing blocks.
	if !vm.isReady() {
		vm.snowCtx.Log.Warn("not building block", zap.Error(ErrNotReady))
		return nil, ErrNotReady
	}

	// Notify builder if we should build again (whether or not we are successful this time)
	//
	// Note: builder should regulate whether or not it actually decides to build based on state
	// of the mempool.
	defer vm.checkActivity(ctx)

	vm.verifiedL.RLock()
	processingBlocks := len(vm.verifiedBlocks)
	vm.verifiedL.RUnlock()
	if processingBlocks > vm.config.GetProcessingBuildSkip() {
		vm.snowCtx.Log.Warn("not building block", zap.Error(ErrTooManyProcessing))
		return nil, ErrTooManyProcessing
	}

	// Build block and store as parsed
	preferredBlk, err := vm.GetStatelessBlock(ctx, vm.preferred)
	if err != nil {
		vm.snowCtx.Log.Warn("unable to get preferred block", zap.Error(err))
		return nil, err
	}
	blk, err := chain.BuildBlock(ctx, vm, preferredBlk)
	if err != nil {
		// This is a DEBUG log because BuildBlock may fail before
		// the min build gap (especially when there are no transactions).
		vm.snowCtx.Log.Debug("BuildBlock failed", zap.Error(err))
		return nil, err
	}
	vm.parsedBlocks.Put(blk.ID(), blk)
	return blk, nil
}

func (vm *VM) Submit(
	ctx context.Context,
	verifyAuth bool,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// We should not allow any transactions to be submitted if the VM is not
	// ready yet. We should never reach this point because of other checks but it
	// is good to be defensive.
	if !vm.isReady() {
		return []error{ErrNotReady}
	}

	// Create temporary execution context
	blk, err := vm.GetStatelessBlock(ctx, vm.preferred)
	if err != nil {
		return []error{err}
	}
	view, err := blk.View(ctx, false)
	if err != nil {
		// This will error if a block does not yet have processed state.
		return []error{err}
	}
	feeRaw, err := view.GetValue(ctx, chain.FeeKey(vm.StateManager().FeeKey()))
	if err != nil {
		return []error{err}
	}
	feeManager := fees.NewManager(feeRaw)
	now := time.Now().UnixMilli()
	r := vm.c.Rules(now)
	nextFeeManager, err := feeManager.ComputeNext(blk.Tmstmp, now, r)
	if err != nil {
		return []error{err}
	}

	// Find repeats
	oldestAllowed := now - r.GetValidityWindow()
	repeats, err := blk.IsRepeat(ctx, oldestAllowed, txs, set.NewBits(), true)
	if err != nil {
		return []error{err}
	}

	validTxs := []*chain.Transaction{}
	for i, tx := range txs {
		// Check if transaction is a repeat before doing any extra work
		if repeats.Contains(i) {
			errs = append(errs, chain.ErrDuplicateTx)
			continue
		}

		// Avoid any sig verification or state lookup if we already have tx in mempool
		txID := tx.ID()
		if vm.mempool.Has(ctx, txID) {
			// Don't remove from listeners, it will be removed elsewhere if not
			// included
			errs = append(errs, ErrNotAdded)
			continue
		}

		// Ensure state keys are valid
		_, err := tx.StateKeys(vm.c.StateManager())
		if err != nil {
			errs = append(errs, ErrNotAdded)
			continue
		}

		// Verify auth if not already verified by caller
		if verifyAuth && vm.config.GetVerifyAuth() {
			msg, err := tx.Digest()
			if err != nil {
				// Should never fail
				errs = append(errs, err)
				continue
			}
			if err := tx.Auth.Verify(ctx, msg); err != nil {
				// Failed signature verification is the only safe place to remove
				// a transaction in listeners. Every other case may still end up with
				// the transaction in a block.
				if err := vm.webSocketServer.RemoveTx(txID, err); err != nil {
					vm.snowCtx.Log.Warn("unable to remove tx from webSocketServer", zap.Error(err))
				}
				errs = append(errs, err)
				continue
			}
		}

		// PreExecute does not make any changes to state
		//
		// This may fail if the state we are utilizing is invalidated (if a trie
		// view from a different branch is committed underneath it). We prefer this
		// instead of putting a lock around all commits.
		//
		// Note, [PreExecute] ensures that the pending transaction does not have
		// an expiry time further ahead than [ValidityWindow]. This ensures anything
		// added to the [Mempool] is immediately executable.
		if err := tx.PreExecute(ctx, nextFeeManager, vm.c.StateManager(), r, view, now); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, nil)
		validTxs = append(validTxs, tx)
	}
	vm.mempool.Add(ctx, validTxs)
	vm.checkActivity(ctx)
	vm.metrics.mempoolSize.Set(float64(vm.mempool.Len(ctx)))
	return errs
}

// "SetPreference" implements "block.ChainVM"
// replaces "core.SnowmanVM.SetPreference"
func (vm *VM) SetPreference(_ context.Context, id ids.ID) error {
	vm.snowCtx.Log.Debug("set preference", zap.Stringer("id", id))
	vm.preferred = id
	return nil
}

// "LastAccepted" implements "block.ChainVM"
// replaces "core.SnowmanVM.LastAccepted"
func (vm *VM) LastAccepted(_ context.Context) (ids.ID, error) {
	return vm.lastAccepted.ID(), nil
}

// Handles incoming "AppGossip" messages, parses them to transactions,
// and submits them to the mempool. The "AppGossip" message is sent by
// the other VM  via "common.AppSender" to receive txs and
// forward them to the other node (validator).
//
// implements "snowmanblock.ChainVM.commom.VM.AppHandler"
// assume gossip via proposervm has been activated
// ref. "avalanchego/vms/platformvm/network.AppGossip"
func (vm *VM) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppGossip")
	defer span.End()

	return vm.networkManager.AppGossip(ctx, nodeID, msg)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppRequest")
	defer span.End()

	return vm.networkManager.AppRequest(ctx, nodeID, requestID, deadline, request)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32, _ *common.AppError) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppRequestFailed")
	defer span.End()

	// TODO: add support for handling common.AppError
	return vm.networkManager.AppRequestFailed(ctx, nodeID, requestID)
}

// implements "block.ChainVM.commom.VM.AppHandler"
func (vm *VM) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppResponse")
	defer span.End()

	return vm.networkManager.AppResponse(ctx, nodeID, requestID, response)
}

func (vm *VM) CrossChainAppRequest(
	ctx context.Context,
	nodeID ids.ID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.CrossChainAppRequest")
	defer span.End()

	return vm.networkManager.CrossChainAppRequest(ctx, nodeID, requestID, deadline, request)
}

func (vm *VM) CrossChainAppRequestFailed(
	ctx context.Context,
	nodeID ids.ID,
	requestID uint32,
	_ *common.AppError,
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.CrossChainAppRequestFailed")
	defer span.End()

	// TODO: add support for handling common.AppError
	return vm.networkManager.CrossChainAppRequestFailed(ctx, nodeID, requestID)
}

func (vm *VM) CrossChainAppResponse(
	ctx context.Context,
	nodeID ids.ID,
	requestID uint32,
	response []byte,
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.CrossChainAppResponse")
	defer span.End()

	return vm.networkManager.CrossChainAppResponse(ctx, nodeID, requestID, response)
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (vm *VM) Connected(ctx context.Context, nodeID ids.NodeID, v *version.Application) error {
	ctx, span := vm.tracer.Start(ctx, "VM.Connected")
	defer span.End()

	return vm.networkManager.Connected(ctx, nodeID, v)
}

// implements "block.ChainVM.commom.VM.validators.Connector"
func (vm *VM) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	ctx, span := vm.tracer.Start(ctx, "VM.Disconnected")
	defer span.End()

	return vm.networkManager.Disconnected(ctx, nodeID)
}

// VerifyHeightIndex implements snowmanblock.HeightIndexedChainVM
func (*VM) VerifyHeightIndex(context.Context) error { return nil }

// GetBlockIDAtHeight implements snowmanblock.HeightIndexedChainVM
// Note: must return database.ErrNotFound if the index at height is unknown.
//
// This is called by the VM pre-ProposerVM fork and by the sync server
// in [GetStateSummary].
func (vm *VM) GetBlockIDAtHeight(_ context.Context, height uint64) (ids.ID, error) {
	if height == vm.lastAccepted.Height() {
		return vm.lastAccepted.ID(), nil
	}
	if height == vm.genesisBlk.Height() {
		return vm.genesisBlk.ID(), nil
	}
	if blkID, ok := vm.acceptedBlocksByHeight.Get(height); ok {
		return blkID, nil
	}
	vm.metrics.blocksHeightsFromDisk.Inc()
	return vm.GetBlockHeightID(height)
}

// backfillSeenTransactions makes a best effort to populate [vm.seen]
// with whatever transactions we already have on-disk. This will lead
// a node to becoming ready faster during a restart.
func (vm *VM) backfillSeenTransactions() {
	// Exit early if we don't have any blocks other than genesis (which
	// contains no transactions)
	blk := vm.lastAccepted
	if blk.Hght == 0 {
		vm.snowCtx.Log.Info("no seen transactions to backfill")
		vm.startSeenTime = 0
		vm.seenValidityWindowOnce.Do(func() {
			close(vm.seenValidityWindow)
		})
		return
	}

	// Backfill [vm.seen] with lifeline worth of transactions
	r := vm.Rules(vm.lastAccepted.Tmstmp)
	oldest := uint64(0)
	for {
		if vm.lastAccepted.Tmstmp-blk.Tmstmp > r.GetValidityWindow() {
			// We are assured this function won't be running while we accept
			// a block, so we don't need to protect against closing this channel
			// twice.
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// It is ok to add transactions from newest to oldest
		vm.seen.Add(blk.Txs)
		vm.startSeenTime = blk.Tmstmp
		oldest = blk.Hght

		// Exit early if next block to fetch is genesis (which contains no
		// txs)
		if blk.Hght <= 1 {
			// If we have walked back from the last accepted block to genesis, then
			// we can be sure we have all required transactions to start validation.
			vm.startSeenTime = 0
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// Set next blk in lookback
		tblk, err := vm.GetStatelessBlock(context.Background(), blk.Prnt)
		if err != nil {
			vm.snowCtx.Log.Info("could not load block, exiting backfill",
				zap.Uint64("height", blk.Height()-1),
				zap.Stringer("blockID", blk.Prnt),
				zap.Error(err),
			)
			return
		}
		blk = tblk
	}
	vm.snowCtx.Log.Info(
		"backfilled seen txs",
		zap.Uint64("start", oldest),
		zap.Uint64("finish", vm.lastAccepted.Hght),
	)
}

func (vm *VM) loadAcceptedBlocks(ctx context.Context) error {
	start := uint64(0)
	lookback := uint64(vm.config.GetAcceptedBlockWindowCache()) - 1 // include latest
	if vm.lastAccepted.Hght > lookback {
		start = vm.lastAccepted.Hght - lookback
	}
	for i := start; i <= vm.lastAccepted.Hght; i++ {
		blk, err := vm.GetDiskBlock(ctx, i)
		if err != nil {
			vm.snowCtx.Log.Info("could not find block on-disk", zap.Uint64("height", i))
			continue
		}
		vm.acceptedBlocksByID.Put(blk.ID(), blk)
		vm.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	}
	vm.snowCtx.Log.Info("loaded blocks from disk",
		zap.Uint64("start", start),
		zap.Uint64("finish", vm.lastAccepted.Hght),
	)
	return nil
}

// Fatal logs the provided message and then panics to force an exit.
//
// While we could attempt a graceful shutdown, it is not clear that
// the shutdown will complete given that we have encountered a fatal
// issue. It is better to ensure we exit to surface the error.
func (vm *VM) Fatal(msg string, fields ...zap.Field) {
	vm.snowCtx.Log.Fatal(msg, fields...)
	panic("fatal error")
}
