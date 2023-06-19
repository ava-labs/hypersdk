// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/x/merkledb"
	syncEng "github.com/ava-labs/avalanchego/x/sync"
	hcache "github.com/ava-labs/hypersdk/cache"
	"github.com/prometheus/client_golang/prometheus"

	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/rpc"
	htrace "github.com/ava-labs/hypersdk/trace"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/workers"
)

type VM struct {
	c Controller
	v *version.Semantic

	snowCtx         *snow.Context
	pkBytes         []byte
	proposerMonitor *ProposerMonitor
	manager         manager.Manager

	config  Config
	genesis Genesis

	builder    builder.Builder
	builtBlock *chain.StatelessRootBlock
	building   bool

	gossiper       gossiper.Gossiper
	rawStateDB     database.Database
	stateDB        merkledb.MerkleDB
	vmDB           database.Database
	handlers       Handlers
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry

	tracer  trace.Tracer
	mempool *mempool.Mempool[*chain.Transaction]

	// track all accepted but still valid txs (replay protection)
	seen                   *emap.EMap[*chain.Transaction]
	startSeenTime          int64
	seenValidityWindowOnce sync.Once
	seenValidityWindow     chan struct{}

	// cache block objects to optimize "GetBlockStateless"
	// only put when a block is accepted
	blocks *hcache.FIFO[ids.ID, *chain.StatelessRootBlock]

	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *cache.LRU[ids.ID, *chain.StatelessRootBlock]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL      sync.RWMutex
	verifiedBlocks map[ids.ID]*chain.StatelessRootBlock

	// Accepted block queue
	acceptedQueue chan *chain.StatelessRootBlock
	acceptorDone  chan struct{}

	// Transactions that streaming users are currently subscribed to
	webSocketServer *rpc.WebSocketServer

	// Reuse gorotuine group to avoid constant re-allocation
	workers *workers.Workers

	bootstrapped  utils.Atomic[bool]
	preferred     ids.ID
	lastAccepted  *chain.StatelessRootBlock
	lastProcessed *chain.StatelessRootBlock
	toEngine      chan<- common.Message

	// State Sync client and AppRequest handlers
	stateSyncClient        *stateSyncerClient
	stateSyncNetworkClient syncEng.NetworkClient
	stateSyncNetworkServer *syncEng.NetworkServer

	// Warp manager fetches signatures from other validators for a given accepted
	// txID
	warpManager *WarpManager

	// TxBlockManager distributes and fetches txBlocks
	txBlockManager *TxBlockManager

	// Network manager routes p2p messages to pre-registered handlers
	networkManager *NetworkManager

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
	manager manager.Manager,
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
	gatherer := ametrics.NewMultiGatherer()
	if err := vm.snowCtx.Metrics.Register(gatherer); err != nil {
		return err
	}
	defaultRegistry, metrics, err := newMetrics()
	if err != nil {
		return err
	}
	if err := gatherer.Register("hyper_sdk", defaultRegistry); err != nil {
		return err
	}
	vm.metrics = metrics

	vm.proposerMonitor = NewProposerMonitor(vm)
	vm.networkManager = NewNetworkManager(vm, appSender)

	warpHandler, warpSender := vm.networkManager.Register()
	vm.warpManager = NewWarpManager(vm)
	vm.networkManager.SetHandler(warpHandler, NewWarpHandler(vm))
	go vm.warpManager.Run(warpSender)

	txBlockHandler, txBlockSender := vm.networkManager.Register()
	vm.txBlockManager = NewTxBlockManager(vm)
	vm.networkManager.SetHandler(txBlockHandler, NewTxBlockHandler(vm))
	go vm.txBlockManager.Run(txBlockSender)

	vm.manager = manager

	// Always initialize implementation first
	vm.config, vm.genesis, vm.builder, vm.gossiper, vm.vmDB,
		vm.rawStateDB, vm.handlers, vm.actionRegistry, vm.authRegistry, err = vm.c.Initialize(
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
	vm.tracer, err = htrace.New(vm.config.GetTraceConfig())
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
		HistoryLength: vm.config.GetStateHistoryLength(),
		NodeCacheSize: vm.config.GetStateCacheSize(),
		Reg:           merkleRegistry,
		Tracer:        vm.tracer,
	})
	if err != nil {
		return err
	}
	if err := gatherer.Register("state", merkleRegistry); err != nil {
		return err
	}

	// Setup worker cluster
	vm.workers = workers.New(vm.config.GetParallelism(), 100)

	// Init channels before initializing other structs
	vm.toEngine = toEngine

	vm.blocks, err = hcache.NewFIFO[ids.ID, *chain.StatelessRootBlock](vm.config.GetAcceptedBlockCacheSize())
	if err != nil {
		return err
	}
	vm.acceptedQueue = make(chan *chain.StatelessRootBlock, vm.config.GetAcceptorSize())
	vm.acceptorDone = make(chan struct{})
	vm.parsedBlocks = &cache.LRU[ids.ID, *chain.StatelessRootBlock]{Size: vm.config.GetParsedBlockCacheSize()}
	vm.verifiedBlocks = make(map[ids.ID]*chain.StatelessRootBlock)

	mem, mempoolRegistry, err := mempool.New[*chain.Transaction](
		vm.tracer,
		vm.config.GetMempoolSize(),
	)
	if err != nil {
		return err
	}
	if err := gatherer.Register("mempool", mempoolRegistry); err != nil {
		return err
	}
	vm.mempool = mem

	// Try to load last accepted
	has, err := vm.HasLastAccepted()
	if err != nil {
		snowCtx.Log.Error("could not determine if have last accepted")
		return err
	}
	if has { //nolint:nestif
		blkID, err := vm.GetLastAccepted()
		if err != nil {
			snowCtx.Log.Error("could not get last accepted", zap.Error(err))
			return err
		}

		blk, err := vm.GetStatelessRootBlock(ctx, blkID)
		if err != nil {
			snowCtx.Log.Error("could not load last accepted", zap.Error(err))
			return err
		}

		vm.preferred, vm.lastAccepted, vm.lastProcessed = blkID, blk, blk
		snowCtx.Log.Info("initialized vm from last accepted", zap.Stringer("block", blkID))
	} else {
		// Set Balances
		view, err := vm.stateDB.NewView()
		if err != nil {
			return err
		}
		if err := vm.genesis.Load(ctx, vm.tracer, view); err != nil {
			snowCtx.Log.Error("could not set genesis allocation", zap.Error(err))
			return err
		}
		if err := view.Insert(ctx, vm.StateManager().ParentUnitPriceKey(), binary.BigEndian.AppendUint64(nil, vm.Rules(0).GetMinUnitPrice())); err != nil {
			return err
		}
		if err := view.CommitToDB(ctx); err != nil {
			return err
		}
		root, err := vm.stateDB.GetMerkleRoot(ctx)
		if err != nil {
			snowCtx.Log.Error("could not get merkle root", zap.Error(err))
		}
		snowCtx.Log.Debug("genesis state created", zap.Stringer("root", root))

		// Create genesis block
		genesisTxBlk, err := chain.ParseTxBlock(
			ctx,
			chain.NewGenesisTxBlock(),
			nil,
			vm,
		)
		if err != nil {
			snowCtx.Log.Error("unable to init genesis block", zap.Error(err))
			return err
		}
		batch := vm.vmDB.NewBatch()
		if err := vm.StoreTxBlock(batch, genesisTxBlk); err != nil {
			snowCtx.Log.Error("could not store genesis tx blk", zap.Error(err))
			return err
		}
		if err := batch.Write(); err != nil {
			snowCtx.Log.Error("could not commit genesis tx blk", zap.Error(err))
			return err
		}
		genesisBlk, err := chain.ParseRootBlock(
			ctx,
			chain.NewGenesisRootBlock(genesisTxBlk.ID()),
			[]*chain.StatelessTxBlock{genesisTxBlk},
			nil,
			choices.Accepted,
			vm,
		)
		if err != nil {
			snowCtx.Log.Error("unable to init genesis block", zap.Error(err))
			return err
		}
		if err := vm.SetLastAccepted(genesisBlk); err != nil {
			snowCtx.Log.Error("could not set genesis as last accepted", zap.Error(err))
			return err
		}
		gBlkID := genesisBlk.ID()
		vm.preferred, vm.lastAccepted, vm.lastProcessed = gBlkID, genesisBlk, genesisBlk
		vm.blocks.Put(gBlkID, genesisBlk)
		snowCtx.Log.Info("initialized vm from genesis", zap.Stringer("block", gBlkID))
	}
	go vm.processAcceptedBlocks()

	// Setup state syncing
	stateSyncHandler, stateSyncSender := vm.networkManager.Register()
	vm.stateSyncNetworkClient = syncEng.NewNetworkClient(
		stateSyncSender,
		vm.snowCtx.NodeID,
		int64(vm.config.GetStateSyncParallelism()),
		vm.Logger(),
	)
	vm.stateSyncClient = vm.NewStateSyncClient(gatherer)
	vm.stateSyncNetworkServer = syncEng.NewNetworkServer(stateSyncSender, vm.stateDB, vm.Logger())
	vm.networkManager.SetHandler(stateSyncHandler, NewStateSyncHandler(vm))

	// Startup block builder and gossiper
	go vm.builder.Run()
	gossipHandler, gossipSender := vm.networkManager.Register()
	vm.networkManager.SetHandler(gossipHandler, NewTxGossipHandler(vm))
	go vm.gossiper.Run(gossipSender)

	// Wait until VM is ready and then send a state sync message to engine
	go vm.markReady()

	// Setup handlers
	jsonRPCHandler, err := rpc.NewJSONRPCHandler(rpc.Name, rpc.NewJSONRPCServer(vm), common.NoLock)
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
	vm.handlers[rpc.WebSocketEndpoint] = rpc.NewWebSocketHandler(pubsubServer)
	return nil
}

func (vm *VM) markReady() {
	select {
	case <-vm.stop:
		return
	case <-vm.stateSyncClient.done:
	}
	vm.snowCtx.Log.Info("state sync client ready")
	select {
	case <-vm.stop:
		return
	case <-vm.seenValidityWindow:
	}
	vm.snowCtx.Log.Info("validity window ready")
	if vm.stateSyncClient.Started() {
		// only alert engine if we started
		vm.toEngine <- common.StateSyncDone
	}
	close(vm.ready)
	vm.snowCtx.Log.Info(
		"node is now ready",
		zap.Bool("synced", vm.stateSyncClient.Started()),
	)
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

func (vm *VM) Manager() manager.Manager {
	return vm.manager
}

func (vm *VM) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	if !vm.isReady() {
		return hutils.Repeat[[]byte](nil, len(keys)), hutils.Repeat(ErrNotReady, len(keys))
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
		}
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
	vm.warpManager.Done()
	vm.builder.Done()
	vm.gossiper.Done()
	vm.workers.Stop()
	if vm.profiler != nil {
		vm.profiler.Shutdown()
	}
	vm.txBlockManager.Done()

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
func (vm *VM) CreateHandlers(_ context.Context) (map[string]*common.HTTPHandler, error) {
	return vm.handlers, nil
}

// implements "block.ChainVM.common.VM"
// for "ext/vm/[vmID]"
func (*VM) CreateStaticHandlers(_ context.Context) (map[string]*common.HTTPHandler, error) {
	return nil, nil
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
func (vm *VM) GetBlock(ctx context.Context, id ids.ID) (snowman.Block, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.GetBlock")
	defer span.End()

	// We purposely don't return parsed but unverified blocks from here
	return vm.GetStatelessRootBlock(ctx, id)
}

func (vm *VM) GetStatelessRootBlock(ctx context.Context, blkID ids.ID) (*chain.StatelessRootBlock, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.GetStatelessRootBlock")
	defer span.End()

	// has the block been cached from previous "Accepted" call
	blk, exist := vm.blocks.Get(blkID)
	if exist {
		return blk, nil
	}

	// has the block been verified, not yet accepted
	vm.verifiedL.RLock()
	if blk, exists := vm.verifiedBlocks[blkID]; exists {
		vm.verifiedL.RUnlock()
		return blk, nil
	}
	vm.verifiedL.RUnlock()

	// not found in memory, fetch from disk if accepted
	stBlk, err := vm.GetDiskBlock(blkID)
	if err != nil {
		return nil, err
	}
	// If block on disk, it must've been accepted
	return chain.ParseRootBlock(ctx, stBlk, nil, nil, choices.Accepted, vm)
}

// implements "block.ChainVM.commom.VM.Parser"
// replaces "core.SnowmanVM.ParseBlock"
func (vm *VM) ParseBlock(ctx context.Context, source []byte) (snowman.Block, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.ParseBlock")
	defer span.End()

	// Check to see if we've already parsed
	id := hutils.ToID(source)

	// If we have seen this block before, return it with the most
	// up-to-date info
	if oldBlk, err := vm.GetStatelessRootBlock(ctx, id); err == nil {
		vm.snowCtx.Log.Debug("returning previously parsed block", zap.Stringer("id", oldBlk.ID()))
		return oldBlk, nil
	}

	// Attempt to parse and cache block
	blk, exist := vm.parsedBlocks.Get(id)
	if exist {
		return blk, nil
	}
	newBlk, err := chain.ParseStatelessRootBlock(
		ctx,
		nil,
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

func (vm *VM) buildBlock(
	ctx context.Context,
	blockContext *smblock.Context,
) (snowman.Block, error) {
	if !vm.isReady() {
		vm.snowCtx.Log.Warn("not building block", zap.Error(ErrNotReady))
		return nil, ErrNotReady
	}

	if vm.building {
		return nil, errors.New("already building block")
	}
	var builtBlock *chain.StatelessRootBlock
	if vm.builtBlock != nil {
		if vm.builtBlock.Prnt == vm.preferred {
			builtBlock = vm.builtBlock
			vm.snowCtx.Log.Info("found previously built block", zap.Stringer("blkID", builtBlock.ID()))
		} else {
			// TODO: cancel ongoing building in verify
			vm.snowCtx.Log.Warn("discarding previously built block", zap.Stringer("blkID", vm.builtBlock.ID()))
			// Re-add transactions to mempool when block is discarded
			for _, txBlock := range vm.builtBlock.GetTxBlocks() {
				_ = vm.Submit(ctx, false, txBlock.Txs)
			}
		}
		vm.builtBlock = nil
	}
	if builtBlock == nil {
		vm.building = true
		preferred := vm.preferred
		vm.snowCtx.Log.Info("starting async build", zap.Stringer("parent", preferred))
		start := time.Now()
		go func() {
			defer func() {
				vm.building = false
			}()
			blk, err := chain.BuildBlock(ctx, vm, vm.preferred, blockContext)
			if err != nil {
				vm.snowCtx.Log.Warn("BuildBlock failed", zap.Error(err))
				return
			}
			if blk.Prnt != vm.preferred {
				vm.snowCtx.Log.Warn("abandoning built blk", zap.Error(err))
				for _, txBlock := range blk.GetTxBlocks() {
					_ = vm.Submit(ctx, false, txBlock.Txs)
				}
				return
			}
			vm.builtBlock = blk
			dur := time.Since(start)
			vm.snowCtx.Log.Info("built block async", zap.Duration("t", dur))
			vm.metrics.buildBlock.Observe(float64(dur))
			vm.builder.TriggerBuild()
		}()
		// Now that we do a 250ms wait, we don't need this additional sleep
		// vm.builder.HandleGenerateBlock()
		return nil, errors.New("building block")
	}
	vm.parsedBlocks.Put(builtBlock.ID(), builtBlock)
	return builtBlock, nil
}

// implements "block.ChainVM"
func (vm *VM) BuildBlock(ctx context.Context) (snowman.Block, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.BuildBlock")
	defer span.End()

	return vm.buildBlock(ctx, nil)
}

// implements "block.BuildBlockWithContextChainVM"
func (vm *VM) BuildBlockWithContext(
	ctx context.Context,
	blockContext *smblock.Context,
) (snowman.Block, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.BuildBlockWithContext")
	defer span.End()

	return vm.buildBlock(ctx, blockContext)
}

// TODO: batch submit to better amortize block context overhead
func (vm *VM) Submit(
	ctx context.Context,
	verifySig bool,
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
	// TODO: use last processed block here instead of preferred
	// TODO: just verify signatures and no duplicates here?
	blk, err := vm.PreferredBlock(ctx)
	if err != nil {
		return []error{err}
	}
	txBlk, err := blk.LastTxBlock() // could be nil if last accepted is genesis
	if err != nil {
		return []error{err}
	}
	now := time.Now().Unix()
	r := vm.c.Rules(now)
	oldestAllowed := now - r.GetValidityWindow()
	validTxs := []*chain.Transaction{}
	for _, tx := range txs {
		txID := tx.ID()
		// We already verify in streamer, let's avoid re-verification
		if verifySig && vm.config.GetVerifySignatures() {
			sigVerify := tx.AuthAsyncVerify()
			if err := sigVerify(); err != nil {
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
		// TODO: do we need this? (just ensures people can't spam mempool with
		// txs from already verified blocks)
		if txBlk != nil {
			repeat, err := txBlk.IsRepeat(ctx, oldestAllowed, []*chain.Transaction{tx})
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if repeat {
				errs = append(errs, chain.ErrDuplicateTx)
				continue
			}
		}
		errs = append(errs, nil)
		validTxs = append(validTxs, tx)
	}
	vm.mempool.Add(ctx, validTxs)
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
func (vm *VM) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32) error {
	ctx, span := vm.tracer.Start(ctx, "VM.AppRequestFailed")
	defer span.End()

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
) error {
	ctx, span := vm.tracer.Start(ctx, "VM.CrossChainAppRequestFailed")
	defer span.End()

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
func (vm *VM) GetBlockIDAtHeight(_ context.Context, height uint64) (ids.ID, error) {
	return vm.GetDiskBlockIDAtHeight(height)
}
