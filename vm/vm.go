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

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
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
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/version"
	hcache "github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/filedb"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/vilmo"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/network"
	"github.com/ava-labs/hypersdk/rpc"
	htrace "github.com/ava-labs/hypersdk/trace"
	hutils "github.com/ava-labs/hypersdk/utils"
)

type executedWrapper struct {
	Block      *chain.StatefulBlock
	Chunk      *chain.FilteredChunk
	Results    []*chain.Result
	InvalidTxs []ids.ID
}

type acceptedWrapper struct {
	Block          *chain.StatelessBlock
	FilteredChunks []*chain.FilteredChunk
}

type VM struct {
	c Controller
	v *version.Semantic

	snowCtx         *snow.Context
	pkBytes         []byte
	proposerMonitor *ProposerMonitor

	config         Config
	genesis        Genesis
	vmDB           database.Database
	blobDB         *filedb.FileDB
	stateDB        *vilmo.Vilmo
	handlers       Handlers
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry
	authEngine     map[uint8]AuthEngine

	tracer trace.Tracer

	// Handle chunks
	cm     *ChunkManager
	engine *chain.Engine

	// track all issuedTxs (to prevent wasting bandwidth)
	//
	// we use an emap here to avoid recursing through all previously
	// issued chunks when packing a new chunk.
	issuedTxs        *emap.LEMap[*chain.Transaction]
	rpcAuthorizedTxs *emap.LEMap[*chain.Transaction] // used to optimize performance of RPCs
	mempool          *mempool.Mempool[*chain.Transaction]

	// track all accepted but still valid txs (replay protection)
	seenTxs                *emap.LEMap[*chain.Transaction]
	seenChunks             *emap.LEMap[*chain.ChunkCertificate]
	startSeenTime          int64
	seenValidityWindowOnce sync.Once
	seenValidityWindow     chan struct{}

	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *cache.LRU[ids.ID, *chain.StatelessBlock]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL      sync.RWMutex
	verifiedBlocks map[ids.ID]*chain.StatelessBlock

	// We store the last [AcceptedBlockWindowCache] blocks in memory
	// to avoid reading blocks from disk.
	acceptedBlocksByID     *hcache.FIFO[ids.ID, *chain.StatelessBlock]
	acceptedBlocksByHeight *hcache.FIFO[uint64, ids.ID]

	// Executed chunk queue
	executedQueue chan *executedWrapper
	executorDone  chan struct{}

	// Accepted block queue
	acceptedQueue chan *acceptedWrapper
	acceptorDone  chan struct{}

	// Transactions that streaming users are currently subscribed to
	webSocketServer *rpc.WebSocketServer

	bootstrapped utils.Atomic[bool]
	genesisBlk   *chain.StatelessBlock
	preferred    ids.ID
	lastAccepted *chain.StatelessBlock
	toEngine     chan<- common.Message

	// Warp manager fetches signatures from other validators for a given accepted
	// txID
	warpManager *WarpManager

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
	_ database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
	appSender common.AppSender,
) error {
	vm.snowCtx = snowCtx
	vm.pkBytes = bls.PublicKeyToCompressedBytes(vm.snowCtx.PublicKey)
	vm.issuedTxs = emap.NewLEMap[*chain.Transaction]()
	vm.rpcAuthorizedTxs = emap.NewLEMap[*chain.Transaction]()
	// This will be overwritten when we accept the first block (in state sync) or
	// backfill existing blocks (during normal bootstrapping).
	vm.startSeenTime = -1
	// Init seen for tracking transactions that have been accepted on-chain
	vm.seenTxs = emap.NewLEMap[*chain.Transaction]()
	vm.seenChunks = emap.NewLEMap[*chain.ChunkCertificate]()
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
	if err := gatherer.Register("hypersdk", defaultRegistry); err != nil {
		return err
	}
	vm.metrics = metrics

	// Initialize the user-provided VM
	vm.config, vm.genesis, vm.handlers, vm.actionRegistry, vm.authRegistry, vm.authEngine, err = vm.c.Initialize(
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

	// Setup network and validator tracking
	vm.networkManager = network.NewManager(vm.snowCtx.Log, vm.snowCtx.NodeID, appSender)
	vm.proposerMonitor = NewProposerMonitor(vm)

	// Initialize warp handler
	warpHandler, warpSender := vm.networkManager.Register()
	vm.warpManager = NewWarpManager(vm)
	vm.networkManager.SetHandler(warpHandler, NewWarpHandler(vm))
	go vm.warpManager.Run(warpSender)

	// Initialize chunk manager
	chunkHandler, chunkSender := vm.networkManager.Register()
	vm.cm = NewChunkManager(vm)
	vm.networkManager.SetHandler(chunkHandler, vm.cm)
	go vm.cm.Run(chunkSender)

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
	vmCfg := pebble.NewDefaultConfig()
	vmCfg.Sync = true
	vmPath, err := hutils.InitSubDirectory(snowCtx.ChainDataDir, "vmdb")
	if err != nil {
		return err
	}
	vmDB, vmDBRegistry, err := pebble.New(vmPath, vmCfg)
	if err != nil {
		return err
	}
	if gatherer != nil {
		if err := gatherer.Register("vmdb", vmDBRegistry); err != nil {
			return err
		}
	}
	vm.vmDB = vmDB
	blobPath, err := hutils.InitSubDirectory(snowCtx.ChainDataDir, "blobdb")
	if err != nil {
		return err
	}
	blobDB := filedb.New(blobPath, true, 1024, 1*units.GiB) // TODO: make configurable
	if err != nil {
		return err
	}
	vm.blobDB = blobDB
	statePath, err := hutils.InitSubDirectory(snowCtx.ChainDataDir, "statedb")
	if err != nil {
		return err
	}
	stateDB, _, err := vilmo.New(vm.Logger(), statePath, 15_000_000, 50_000 /* default batch size */, 64*units.KiB, 256 /* history */) // TODO: make these configs
	if err != nil {
		return err
	}
	// TODO: rollback to last checksum on unclean shutdown
	vm.stateDB = stateDB

	// Track size of [chainData]
	go vm.trackChainDataSize()

	// Init channels before initializing other structs
	vm.toEngine = toEngine

	vm.parsedBlocks = &cache.LRU[ids.ID, *chain.StatelessBlock]{Size: vm.config.GetParsedBlockCacheSize()}
	vm.verifiedBlocks = make(map[ids.ID]*chain.StatelessBlock)
	vm.acceptedBlocksByID, err = hcache.NewFIFO[ids.ID, *chain.StatelessBlock](vm.config.GetAcceptedBlockWindowCache())
	if err != nil {
		return err
	}
	vm.acceptedBlocksByHeight, err = hcache.NewFIFO[uint64, ids.ID](vm.config.GetAcceptedBlockWindowCache())
	if err != nil {
		return err
	}
	vm.executedQueue = make(chan *executedWrapper, vm.config.GetAcceptorSize())
	vm.executorDone = make(chan struct{})
	vm.acceptedQueue = make(chan *acceptedWrapper, vm.config.GetAcceptorSize())
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
		// Prepare Vilmo
		batch, err := stateDB.NewBatch()
		if err != nil {
			return err
		}
		batch.Prepare() // should be a no-op

		// Set balances and compute genesis root
		if err := vm.genesis.Load(ctx, vm.tracer, batch); err != nil {
			snowCtx.Log.Error("could not set genesis allocation", zap.Error(err))
			return err
		}

		// Update chain metadata
		if err := batch.Put(ctx, chain.HeightKey(vm.StateManager().HeightKey()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
			return err
		}
		if err := batch.Put(ctx, chain.TimestampKey(vm.StateManager().TimestampKey()), binary.BigEndian.AppendUint64(nil, uint64(chain.GenesisTime))); err != nil {
			return err
		}

		// Commit genesis block post-execution state and compute root
		checksum, err := batch.Write()
		if err != nil {
			return err
		}

		// Create genesis block
		genesisBlk, err := chain.ParseStatefulBlock(
			ctx,
			chain.NewGenesisBlock(checksum),
			nil,
			choices.Accepted,
			vm,
		)
		if err != nil {
			snowCtx.Log.Error("unable to init genesis block", zap.Error(err))
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
			zap.Stringer("checksum", checksum),
		)
	}
	go vm.processExecutedChunks()
	go vm.processAcceptedBlocks()

	// Setup chain engine
	vm.engine = chain.NewEngine(vm, 128)
	go vm.engine.Run()

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

func (vm *VM) trackChainDataSize() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			start := time.Now()
			size := hutils.DirectorySize(vm.snowCtx.ChainDataDir)
			vm.metrics.chainDataSize.Set(float64(size))
			vm.snowCtx.Log.Info("chainData size", zap.String("size", humanize.Bytes(size)), zap.Duration("t", time.Since(start)))

			keys, aliveBytes, uselessBytes := vm.stateDB.Usage()
			vm.metrics.appendDBKeys.Set(float64(keys))
			vm.metrics.appendDBAliveBytes.Set(float64(aliveBytes))
			vm.metrics.appendDBUselessBytes.Set(float64(uselessBytes))
			vm.snowCtx.Log.Info(
				"stateDB size",
				zap.Int("len", keys),
				zap.String("alive", humanize.Bytes(uint64(aliveBytes))),
				zap.String("useless", humanize.Bytes(uint64(uselessBytes))),
			)
		case <-vm.stop:
			return
		}
	}
}

func (vm *VM) markReady() {
	// Wait for a full [ValidityWindow] before
	// we are willing to vote on blocks.
	select {
	case <-vm.stop:
		return
	case <-vm.seenValidityWindow:
	}
	vm.snowCtx.Log.Info("validity window ready")
	close(vm.ready)

	// Mark node ready and attempt to build a block.
	vm.snowCtx.Log.Info("node is now ready")
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

// ReadState reads the latest executed state
func (vm *VM) ReadState(ctx context.Context, keys []string) ([][]byte, []error) {
	if !vm.isReady() {
		return hutils.Repeat[[]byte](nil, len(keys)), hutils.Repeat(ErrNotReady, len(keys))
	}

	return vm.engine.ReadLatestState(ctx, keys)
}

func (vm *VM) SetState(_ context.Context, state snow.State) error {
	switch state {
	case snow.Bootstrapping:
		// Backfill seen transactions, if any. This will exit as soon as we reach
		// a block we no longer have on disk or if we have walked back the full
		// [ValidityWindow].
		vm.backfillSeenTransactions()

		// Trigger that bootstrapping has started
		vm.Logger().Info("bootstrapping started")
		return vm.onBootstrapStarted()
	case snow.NormalOp:
		vm.Logger().
			Info("normal operation started")
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

	// Shutdown engine
	vm.engine.Done()

	// Process remaining executed chunks before shutdown
	close(vm.executedQueue)
	<-vm.executorDone

	// Process remaining accepted blocks before shutdown
	close(vm.acceptedQueue)
	<-vm.acceptorDone

	// Shutdown other async VM mechanisms
	vm.warpManager.Done()
	vm.cm.Done()
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
	errs := wrappers.Errs{}
	errs.Add(
		vm.vmDB.Close(),
		vm.stateDB.Close(),
		vm.blobDB.Close(),
	)
	return errs.Err
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
		return nil, fmt.Errorf("%w: can't get block height by ID (%s)", err, blkID)
	}
	// We wait to count this metric until we know we have
	// the index on-disk because peers may query us for
	// blocks we don't have yet at tip and we don't want
	// to count that as a historical read.
	vm.metrics.blocksFromDisk.Inc()
	blk, err := vm.GetDiskBlock(ctx, blkHeight)
	if err != nil {
		return nil, fmt.Errorf("%w: can't get block from disk", err)
	}
	return blk, nil
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
	id := hutils.ToID(source)

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
		zap.Uint64("height", newBlk.Height()),
	)
	return newBlk, nil
}

// implements "block.ChainVM"
func (vm *VM) BuildBlock(ctx context.Context) (snowman.Block, error) {
	// The first block of any ProposerVM chain is a pre-fork block: https://github.com/ava-labs/avalanchego/blob/f546ca45621061c0058887cd248cd020065cd7f9/vms/proposervm/vm.go#L233-L236
	vm.snowCtx.Log.Warn("building block without context")
	return vm.buildBlock(ctx, 0)
}

// implements "block.BuildBlockWithContextChainVM"
func (vm *VM) BuildBlockWithContext(ctx context.Context, blockContext *smblock.Context) (snowman.Block, error) {
	return vm.buildBlock(ctx, blockContext.PChainHeight)
}

func (vm *VM) buildBlock(ctx context.Context, pChainHeight uint64) (snowman.Block, error) {
	start := time.Now()
	defer func() {
		vm.metrics.blockBuild.Observe(float64(time.Since(start)))
	}()

	ctx, span := vm.tracer.Start(ctx, "VM.buildBlock")
	defer span.End()

	// If the node isn't ready, we should exit.
	//
	// We call [QueueNotify] when the VM becomes ready, so exiting
	// early here should not cause us to stop producing blocks.
	if !vm.isReady() {
		vm.snowCtx.Log.Warn("not building block", zap.Error(ErrNotReady))
		return nil, ErrNotReady
	}

	vm.verifiedL.RLock()
	processingBlocks := len(vm.verifiedBlocks)
	vm.verifiedL.RUnlock()
	if processingBlocks > vm.config.GetProcessingBuildSkip() {
		// We specify this separately because we want a lower amount than other VMs
		vm.snowCtx.Log.Warn("not building block", zap.Error(ErrTooManyProcessing))
		return nil, ErrTooManyProcessing
	}

	// Build block and store as parsed
	preferredBlk, err := vm.GetStatelessBlock(ctx, vm.preferred)
	if err != nil {
		vm.snowCtx.Log.Warn("unable to get preferred block", zap.Stringer("preferred", vm.preferred), zap.Error(err))
		return nil, err
	}
	blk, err := chain.BuildBlock(ctx, vm, pChainHeight, preferredBlk)
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

	// TODO: check that tx is in our partition

	// Check for duplicates
	//
	// We don't need to check all seen because we are the exclusive issuer of txs
	// for our partition.
	repeats := vm.issuedTxs.Contains(txs, set.NewBits(), false)

	// Perform basic validity checks before storing in mempool
	now := time.Now().UnixMilli()
	r := vm.Rules(now)
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

		// Perform syntactic verification
		if _, err := tx.SyntacticVerify(ctx, vm.StateManager(), r, now); err != nil {
			errs = append(errs, err)
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

		// TODO: Check that bond is valid
		//
		// Outstanding tx limit is maintained in chunk builder
		// TODO: add immutable access
		// ok, err := vm.StateManager().CanProcess(ctx, tx.Auth.Sponsor(), hutils.Epoch(tx.Base.Timestamp, r.GetEpochDuration()), nil)
		// if err != nil {
		// 	errs = append(errs, err)
		// 	continue
		// }
		// if !ok {
		// 	errs = append(errs, errors.New("sponsor has no valid bond"))
		// 	continue
		// }

		errs = append(errs, nil)
		vm.cm.HandleTx(ctx, tx)
	}
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
	if blk.Height() == 0 {
		vm.snowCtx.Log.Info("no seen transactions to backfill")
		vm.startSeenTime = 0
		vm.seenValidityWindowOnce.Do(func() {
			close(vm.seenValidityWindow)
		})
		return
	}

	// Backfill [vm.seen] with lifeline worth of transactions
	t := vm.lastAccepted.StatefulBlock.Timestamp
	r := vm.Rules(t)
	oldest := uint64(0)
	for {
		if t-blk.StatefulBlock.Timestamp > r.GetValidityWindow() {
			// We are assured this function won't be running while we accept
			// a block, so we don't need to protect against closing this channel
			// twice.
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// Iterate through all filtered chunks in accepted blocks
		//
		// It is ok to add transactions from newest to oldest
		for _, filteredChunk := range blk.ExecutedChunks {
			chunk, err := vm.GetFilteredChunk(filteredChunk)
			if err != nil {
				panic(err)
			}
			vm.seenTxs.Add(chunk.Txs)
		}
		vm.startSeenTime = blk.StatefulBlock.Timestamp
		oldest = blk.Height()

		// Exit early if next block to fetch is genesis (which contains no
		// txs)
		if blk.Height() <= 1 {
			// If we have walked back from the last accepted block to genesis, then
			// we can be sure we have all required transactions to start validation.
			vm.startSeenTime = 0
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// Set next blk in lookback
		tblk, err := vm.GetStatelessBlock(context.Background(), blk.Parent())
		if err != nil {
			vm.snowCtx.Log.Info("could not load block, exiting backfill",
				zap.Uint64("height", blk.Height()-1),
				zap.Stringer("blockID", blk.Parent()),
				zap.Error(err),
			)
			return
		}
		blk = tblk
	}
	vm.snowCtx.Log.Info(
		"backfilled seen txs",
		zap.Uint64("start", oldest),
		zap.Uint64("finish", vm.lastAccepted.Height()),
	)
}

func (vm *VM) loadAcceptedBlocks(ctx context.Context) error {
	start := uint64(0)
	lookback := uint64(vm.config.GetAcceptedBlockWindowCache()) - 1 // include latest
	if vm.lastAccepted.Height() > lookback {
		start = vm.lastAccepted.Height() - lookback
	}
	for i := start; i <= vm.lastAccepted.Height(); i++ {
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
		zap.Uint64("finish", vm.lastAccepted.Height()),
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
