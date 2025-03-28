// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chainindex"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/builder"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validators"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/ava-labs/hypersdk/storage"
	"github.com/ava-labs/hypersdk/x/dsmr"
	"github.com/ava-labs/hypersdk/x/fortification"

	avatrace "github.com/ava-labs/avalanchego/trace"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
	hsnow "github.com/ava-labs/hypersdk/snow"
)

const (
	blockDB                 = "blockdb"
	stateDB                 = "statedb"
	chunkDBStr              = "dsmrChunkDB" // TODO: align var naming
	resultsDB               = "results"
	lastResultKey           = byte(0)
	syncerDB                = "syncerdb"
	vmDataDir               = "vm"
	hyperNamespace          = "hypervm"
	chainNamespace          = "chain"
	chainIndexNamespace     = "chainindex"
	dsmrChainIndexNamespace = "dsmrChainIndex"
	gossiperNamespace       = "gossiper"

	changeProofHandlerID uint64 = iota + 1
	rangeProofHandlerID
	txGossipHandlerID
	getChunkProtocolID
	broadcastChunkCertProtocolID
	getChunkSignatureProtocolID
)

var ErrNotAdded = errors.New("not added")

// We set the genesis block timestamp to be after the ProposerVM fork activation.
//
// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
// where the child timestamp is rounded down to the nearest second (which may be before
// the timestamp of its parent, which is denoted in milliseconds).
//
// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
// .../vms/proposervm/pre_fork_block.go#L201
var GenesisTime = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

var (
	_ hsnow.Block = (*chain.ExecutionBlock)(nil)
	_ hsnow.Block = (*chain.OutputBlock)(nil)

	_ hsnow.Chain[*dsmr.Block, *dsmr.Block, *chain.OutputBlock] = (*VM)(nil)
	_ hsnow.ChainIndex[*chain.ExecutionBlock]                   = (*chainindex.ChainIndex[*chain.ExecutionBlock])(nil)
	_ hsnow.ChainIndex[*dsmr.EChunkBlock]                       = (*chainindex.ChainIndex[*dsmr.EChunkBlock])(nil)
)

type VM struct {
	snowInput hsnow.ChainInput
	snowApp   *hsnow.VM[*dsmr.Block, *dsmr.Block, *chain.OutputBlock]

	proposerMonitor *validators.ProposerMonitor

	config Config

	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	genesis               genesis.Genesis
	GenesisBytes          []byte
	ruleFactory           chain.RuleFactory

	opts    []Option
	options *Options

	chainState  *PChainState
	txPartition *fortification.TxPartition[*chain.Transaction]

	// Chain index components
	chainIndex              *chainindex.ChainIndex[*chain.ExecutedBlock]
	chainTimeValidityWindow *validitywindow.TimeValidityWindow[*chain.Transaction]
	syncer                  *validitywindow.Syncer[*chain.Transaction]
	assembler               *chain.Assembler
	preExecutor             *chain.PreExecutor

	// TODO: switch and support DSMR block based state sync
	SyncClient *statesync.Client[*chain.ExecutionBlock]

	// DSMR Indexing components
	dsmrChainIndex *chainindex.ChainIndex[*dsmr.Block]
	dsmrNode       *dsmr.Node[*chain.OutputBlock]

	consensusIndex *hsnow.ConsensusIndex[*dsmr.Block, *dsmr.Block, *chain.OutputBlock]

	normalOp atomic.Bool
	builder  builder.Builder
	gossiper gossiper.Gossiper
	mempool  *mempool.Mempool[*chain.Transaction]

	vmAPIHandlerFactories []api.HandlerFactory[api.VM]
	rawStateDB            database.Database
	stateDB               merkledb.MerkleDB
	balanceHandler        chain.BalanceHandler
	metadataManager       chain.MetadataManager
	txParser              chain.Parser
	abi                   abi.ABI
	actionCodec           *codec.TypeParser[chain.Action]
	authCodec             *codec.TypeParser[chain.Auth]
	outputCodec           *codec.TypeParser[codec.Typed]
	authEngine            map[uint8]AuthEngine

	// authVerifiers are used to verify signatures in parallel
	// with limited parallelism
	authVerifiers workers.Workers

	metrics *Metrics

	network *p2p.Network
	snowCtx *snow.Context
	DataDir string
	tracer  avatrace.Tracer
}

func New(
	genesisFactory genesis.GenesisAndRuleFactory,
	balanceHandler chain.BalanceHandler,
	metadataManager chain.MetadataManager,
	actionCodec *codec.TypeParser[chain.Action],
	authCodec *codec.TypeParser[chain.Auth],
	outputCodec *codec.TypeParser[codec.Typed],
	authEngine map[uint8]AuthEngine,
	options ...Option,
) (*VM, error) {
	allocatedNamespaces := set.NewSet[string](len(options))
	for _, option := range options {
		if allocatedNamespaces.Contains(option.Namespace) {
			return nil, fmt.Errorf("namespace %s already allocated", option.Namespace)
		}
		allocatedNamespaces.Add(option.Namespace)
	}
	abi, err := abi.NewABI(actionCodec.GetRegisteredTypes(), outputCodec.GetRegisteredTypes())
	if err != nil {
		return nil, fmt.Errorf("failed to construct ABI: %w", err)
	}

	return &VM{
		balanceHandler:        balanceHandler,
		metadataManager:       metadataManager,
		txParser:              chain.NewTxTypeParser(actionCodec, authCodec),
		abi:                   abi,
		actionCodec:           actionCodec,
		authCodec:             authCodec,
		outputCodec:           outputCodec,
		authEngine:            authEngine,
		genesisAndRuleFactory: genesisFactory,
		opts:                  options,
	}, nil
}

// implements "block.ChainVM.common.VM"
func (vm *VM) Initialize(
	ctx context.Context,
	chainInput hsnow.ChainInput,
	snowApp *hsnow.VM[*dsmr.Block, *dsmr.Block, *chain.OutputBlock],
) (hsnow.ChainIndex[*dsmr.Block], *dsmr.Block, *chain.OutputBlock, bool, error) {
	vm.DataDir = filepath.Join(chainInput.SnowCtx.ChainDataDir, vmDataDir)
	vm.snowCtx = chainInput.SnowCtx
	vm.snowInput = chainInput
	vm.snowApp = snowApp

	vmRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, hyperNamespace)
	if err != nil {
		return nil, nil, nil, false, err
	}
	vm.metrics, err = newMetrics(vmRegistry)
	if err != nil {
		return nil, nil, nil, false, err
	}
	vm.proposerMonitor = validators.NewProposerMonitor(vm, vm.snowCtx)

	vm.network = snowApp.GetNetwork()

	vm.genesis, vm.ruleFactory, err = vm.genesisAndRuleFactory.Load(chainInput.GenesisBytes, chainInput.UpgradeBytes, vm.snowCtx.NetworkID, vm.snowCtx.ChainID)
	vm.GenesisBytes = chainInput.GenesisBytes
	if err != nil {
		return nil, nil, nil, false, err
	}

	vm.config, err = GetVMConfig(chainInput.Config)
	if err != nil {
		return nil, nil, nil, false, err
	}

	vm.tracer = chainInput.Tracer
	ctx, span := vm.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	vm.initMempool(ctx)

	// Set defaults
	vm.options = &Options{}
	for _, Option := range vm.opts {
		opt, err := Option.optionFunc(vm, vm.snowInput.Config.GetRawConfig(Option.Namespace))
		if err != nil {
			return nil, nil, nil, false, err
		}
		opt.apply(vm.options)
	}
	err = vm.applyModuleOptions()
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to apply options : %w", err)
	}

	vm.chainState = NewPChainState(vm.snowCtx.SubnetID, vm.snowCtx.ValidatorState)
	vm.txPartition, err = fortification.NewTxPartition[*chain.Transaction](vm.snowCtx.Log, vm.chainState)
	if err != nil {
		return nil, nil, nil, false, err
	}

	if err := vm.initChain(ctx); err != nil {
		return nil, nil, nil, false, err
	}
	if err := vm.initDSMR(); err != nil {
		return nil, nil, nil, false, err
	}
	if err := vm.initStateSync(ctx); err != nil {
		return nil, nil, nil, false, err
	}
	if err := vm.applyServiceOptions(); err != nil {
		return nil, nil, nil, false, err
	}

	// If the state is not ready (we are mid state sync), return false without attempting to load
	// the latest block.
	stateReady := !vm.SyncClient.MustStateSync()
	if !stateReady {
		return vm.dsmrChainIndex, nil, nil, false, nil
	}

	// Otherwise, load the latest output/accepted block and return them
	latestDSMRBlock, latestOutputBlock, err := vm.initLatestState(ctx)
	if err != nil {
		return nil, nil, nil, false, err
	}
	return vm.dsmrChainIndex, latestDSMRBlock, latestOutputBlock, true, nil
}

func (vm *VM) initLatestState(ctx context.Context) (*dsmr.Block, *chain.OutputBlock, error) {
	lastAcceptedOutputBlock, err := vm.initLastAccepted(ctx)
	if err != nil {
		return nil, nil, err
	}
	lastAcceptedDSMRBlock, err := vm.dsmrChainIndex.GetBlockByHeight(ctx, lastAcceptedOutputBlock.GetHeight())
	if err != nil {
		return nil, nil, err
	}

	return lastAcceptedDSMRBlock, lastAcceptedOutputBlock, nil
}

func (vm *VM) initMempool(ctx context.Context) {
	vm.mempool = mempool.New[*chain.Transaction](vm.tracer, vm.config.MempoolSize, vm.config.MempoolSponsorSize)
	vm.snowApp.AddAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			droppedTxs := vm.mempool.SetMinTimestamp(ctx, b.Tmstmp)
			vm.snowCtx.Log.Debug("dropping expired transactions from mempool",
				zap.Stringer("blkID", b.GetID()),
				zap.Int("numTxs", len(droppedTxs)),
			)
			vm.mempool.Remove(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})

}

func (vm *VM) initChain(ctx context.Context) error {
	// Instantiate DBs
	stateDBRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, stateDB)
	if err != nil {
		return fmt.Errorf("failed to register statedb metrics: %w", err)
	}
	vm.rawStateDB, err = storage.New(pebble.NewDefaultConfig(), vm.snowCtx.ChainDataDir, stateDB, stateDBRegistry)
	if err != nil {
		return err
	}
	vm.stateDB, err = merkledb.New(ctx, vm.rawStateDB, merkledb.Config{
		BranchFactor: vm.genesis.GetStateBranchFactor(),
		// RootGenConcurrency limits the number of goroutines
		// that will be used across all concurrent root generations
		RootGenConcurrency:          uint(vm.config.RootGenerationCores),
		HistoryLength:               uint(vm.config.StateHistoryLength),
		ValueNodeCacheSize:          uint(vm.config.ValueNodeCacheSize),
		IntermediateNodeCacheSize:   uint(vm.config.IntermediateNodeCacheSize),
		IntermediateWriteBufferSize: uint(vm.config.StateIntermediateWriteBufferSize),
		IntermediateWriteBatchSize:  uint(vm.config.StateIntermediateWriteBatchSize),
		Reg:                         stateDBRegistry,
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.tracer,
	})
	if err != nil {
		return err
	}
	vm.snowApp.AddCloser(stateDB, func() error {
		if err := vm.stateDB.Close(); err != nil {
			return fmt.Errorf("failed to close state db: %w", err)
		}
		if err := vm.rawStateDB.Close(); err != nil {
			return fmt.Errorf("failed to close raw state db: %w", err)
		}
		return nil
	})

	chainIndex, err := initChainIndex[*chain.ExecutedBlock](vm.snowApp, vm.snowInput, chain.NewExecutedBlockParser(vm.txParser), chainIndexNamespace)
	if err != nil {
		return err
	}

	vm.chainIndex = chainIndex
	chainIndexAdapter := validitywindow.NewChainIndexAdapter[*chain.ExecutedBlock, *chain.Transaction](
		func(ctx context.Context, blkID ids.ID) (*chain.ExecutedBlock, error) {
			_, span := vm.tracer.Start(ctx, "VM.ChainValidityWindow.GetExecutionBlock")
			defer span.End()

			return vm.chainIndex.GetBlock(ctx, blkID)
		},
	)
	vm.chainTimeValidityWindow = validitywindow.NewTimeValidityWindow(vm.snowCtx.Log, vm.tracer, chainIndexAdapter, func(timestamp int64) int64 {
		return vm.ruleFactory.GetRules(timestamp).GetValidityWindow()
	})
	vm.syncer = validitywindow.NewSyncer(chainIndexAdapter, vm.chainTimeValidityWindow, func(time int64) int64 {
		return vm.ruleFactory.GetRules(time).GetValidityWindow()
	})
	chainRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, chainNamespace)
	if err != nil {
		return fmt.Errorf("failed to make %q registry: %w", chainNamespace, err)
	}
	chainConfig, err := GetChainConfig(vm.snowInput.Config)
	if err != nil {
		return fmt.Errorf("failed to get chain config: %w", err)
	}

	chainMetrics, err := chain.NewMetrics(chainRegistry)
	if err != nil {
		return fmt.Errorf("failed to create chain metrics: %w", err)
	}
	vm.preExecutor = chain.NewPreExecutor(
		vm.ruleFactory,
		vm.chainTimeValidityWindow,
		vm.metadataManager,
		vm.balanceHandler,
	)
	vm.assembler = chain.NewAssembler(
		vm.tracer,
		vm.snowCtx.Log,
		vm.ruleFactory,
		vm.metadataManager,
		vm.balanceHandler,
		vm.chainTimeValidityWindow,
		chainMetrics,
		chainConfig,
		vm.txParser,
	)
	return nil
}

func (vm *VM) initDSMR() error {
	dsmrChainIndex, err := initChainIndex[*dsmr.Block](vm.snowApp, vm.snowInput, dsmr.Parser{}, dsmrChainIndexNamespace)
	if err != nil {
		return err
	}
	vm.dsmrChainIndex = dsmrChainIndex
	dsmrChainIndexAdapter := validitywindow.NewChainIndexAdapter[*dsmr.EChunkBlock, dsmr.EChunk](
		func(ctx context.Context, blkID ids.ID) (*dsmr.EChunkBlock, error) {
			_, span := vm.tracer.Start(ctx, "VM.ChainValidityWindow.GetExecutionBlock")
			defer span.End()

			blk, err := dsmrChainIndex.GetBlock(ctx, blkID)
			if err != nil {
				return nil, err
			}
			return &dsmr.EChunkBlock{Block: blk}, nil
		},
	)
	chunkValidityWindow := validitywindow.NewTimeValidityWindow[dsmr.EChunk](vm.snowCtx.Log, vm.tracer, dsmrChainIndexAdapter, func(timestamp int64) int64 {
		return vm.ruleFactory.GetRules(timestamp).GetValidityWindow()
	})

	chunkDBRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, chunkDBStr)
	if err != nil {
		return fmt.Errorf("failed to register chunkdb metrics: %w", err)
	}
	chunkDB, err := storage.New(pebble.NewDefaultConfig(), vm.snowCtx.ChainDataDir, chunkDBStr, chunkDBRegistry)
	if err != nil {
		return err
	}
	vm.snowApp.AddCloser(chunkDBStr, func() error {
		if err := chunkDB.Close(); err != nil {
			return fmt.Errorf("failed to close chunk db: %w", err)
		}
		return nil
	})
	dsmrNode, err := dsmr.New(
		vm.snowCtx.NodeID,
		vm.snowCtx.Log,
		vm.snowCtx.WarpSigner,
		vm.chainState,
		dsmr.NewDefaultRuleFactory(vm.snowCtx.NetworkID, vm.snowCtx.SubnetID, vm.snowCtx.ChainID), // TODO: make configurable
		chunkDB,
		chunkValidityWindow,
		vm.assembler,
		nil, //TODO: populate last accepted block
		vm.network,
		getChunkProtocolID,
		broadcastChunkCertProtocolID,
		getChunkSignatureProtocolID,
	)
	if err != nil {
		return fmt.Errorf("failed to create DSMR node: %w", err)
	}
	vm.dsmrNode = dsmrNode
	return nil
}

// applyServiceOptions initializes the service options of the VM
// Services include the block subs and APIs (ie. downstream of the VM)
func (vm *VM) applyServiceOptions() error {
	blockSubs := make([]event.Subscription[*chain.ExecutedBlock], len(vm.options.blockSubscriptionFactories))
	for i, factory := range vm.options.blockSubscriptionFactories {
		sub, err := factory.New()
		if err != nil {
			return err
		}
		blockSubs[i] = sub
	}
	executedBlockSub := event.Aggregate(blockSubs...)
	outputBlockSub := event.Map(func(b *chain.OutputBlock) *chain.ExecutedBlock {
		return &chain.ExecutedBlock{
			Block:            b.ExecutionBlock,
			ExecutionResults: b.ExecutionResults,
		}
	}, executedBlockSub)
	vm.snowApp.AddAcceptedSub(outputBlockSub)
	vm.vmAPIHandlerFactories = vm.options.vmAPIHandlerFactories

	for _, apiFactory := range vm.vmAPIHandlerFactories {
		api, err := apiFactory.New(vm)
		if err != nil {
			return fmt.Errorf("failed to initialize api: %w", err)
		}
		vm.snowApp.AddHandler(api.Path, api.Handler)
	}
	return nil
}

func (vm *VM) SetConsensusIndex(consensusIndex *hsnow.ConsensusIndex[*dsmr.Block, *dsmr.Block, *chain.OutputBlock]) {
	vm.consensusIndex = consensusIndex
}

type closerRegistry interface {
	AddCloser(name string, closer func() error)
}

func initChainIndex[T chainindex.Block](
	closeRegistry closerRegistry,
	snowInput hsnow.ChainInput,
	parser chainindex.Parser[T],
	namespace string,
) (*chainindex.ChainIndex[T], error) {
	chainIndexRegistry, err := metrics.MakeAndRegister(snowInput.SnowCtx.Metrics, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to register %s metrics: %w", namespace, err)
	}
	chainStoreDB, err := storage.New(pebble.NewDefaultConfig(), snowInput.SnowCtx.ChainDataDir, namespace, chainIndexRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain index database: %w", err)
	}
	closeRegistry.AddCloser(chainIndexNamespace, func() error {
		if err := chainStoreDB.Close(); err != nil {
			return fmt.Errorf("failed to close chain index %q db: %w", namespace, err)
		}
		return nil
	})
	config, err := GetChainIndexConfig(snowInput.Config, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain index config: %w", err)
	}
	chainIndex, err := chainindex.New[T](snowInput.SnowCtx.Log, chainIndexRegistry, config, parser, chainStoreDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain index: %w", err)
	}
	return chainIndex, nil
}

// initLastAccepted returns the latest output block that has been accepted
//
// Note: initLastAccepted will re-process blocks forward from the latest state height
// to the latest chain index height.
func (vm *VM) initLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	stateHeight, populated, err := vm.extractStateHeight()
	if err != nil {
		return nil, err
	}
	// If the height is not present, we must not have written the genesis state
	// so we write and return the genesis state as the latest accepted block.
	// XXX: after MerkleDB switches to amortizing writes, it's possible "writing"
	// the initial genesis state will be deferred and held in-memory, such that we
	// execute N blocks and have not written any state to disk when we restart.
	// Depending on how MerkleDB handles this, we may want to re-process blocks.
	if !populated {
		return vm.initGenesisAsLastAccepted(ctx)
	}

	// From the latest output block matching the current state height
	blk, err := vm.chainIndex.GetBlockByHeight(ctx, stateHeight)
	if err != nil {
		return nil, err // Failure to fetch block matching the state is fatal
	}
	parentBlock := &chain.OutputBlock{
		ExecutionBlock:   blk.Block,
		View:             vm.stateDB,
		ExecutionResults: blk.ExecutionResults,
	}

	// Fetch the latest height of any indexed block and re-process forward
	// up to that block.
	latestHeight, err := vm.chainIndex.GetLastAcceptedHeight(ctx)
	if err != nil {
		return nil, err
	}
	for blk.GetHeight() < latestHeight {
		nextBlk, err := vm.chainIndex.GetBlockByHeight(ctx, blk.GetHeight()+1)
		if err != nil {
			return nil, err
		}

		parentBlock, err = vm.executeBlock(ctx, parentBlock, nextBlk.Block)
		if err != nil {
			return nil, err
		}
	}
	return parentBlock, nil
}

// extractStateHeight retturns the height reported by the current state and whether
// the height has been populated (should always be populated after writing the genesis state)
func (vm *VM) extractStateHeight() (uint64, bool, error) {
	heightBytes, err := vm.stateDB.Get(chain.HeightKey(vm.metadataManager.HeightPrefix()))
	if err == database.ErrNotFound {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get state height: %w", err)
	}
	stateHeight, err := database.ParseUInt64(heightBytes)
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse state height: %w", err)
	}
	return stateHeight, true, nil
}

// initGenesisAsLastAccepted writes the genesis block state diff to the stateDB, indexes the genesis block,
// and returns the genesis output block.
// This function guarantees that it writes the merkle state diff prior to indexing the block to ensure the
// merkle state is always ahead of the chain index
func (vm *VM) initGenesisAsLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	ts := tstate.New(0)
	tsv := ts.NewView(state.CompletePermissions, vm.stateDB, 0)
	if err := vm.genesis.InitializeState(ctx, vm.tracer, tsv, vm.balanceHandler); err != nil {
		return nil, fmt.Errorf("failed to initialize genesis state: %w", err)
	}

	// Update chain metadata
	if err := tsv.Insert(ctx, chain.HeightKey(vm.metadataManager.HeightPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, fmt.Errorf("failed to set genesis height: %w", err)
	}
	if err := tsv.Insert(ctx, chain.TimestampKey(vm.metadataManager.TimestampPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, fmt.Errorf("failed to set genesis timestamp: %w", err)
	}
	genesisRules := vm.ruleFactory.GetRules(0)
	feeManager := internalfees.NewManager(nil)
	minUnitPrice := genesisRules.GetMinUnitPrice()
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		feeManager.SetUnitPrice(i, minUnitPrice[i])
		vm.snowCtx.Log.Info("set genesis unit price", zap.Int("dimension", int(i)), zap.Uint64("price", feeManager.UnitPrice(i)))
	}
	if err := tsv.Insert(ctx, chain.FeeKey(vm.metadataManager.FeePrefix()), feeManager.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to set genesis fee manager: %w", err)
	}

	// Commit genesis block post-execution state and compute root
	tsv.Commit()
	view, err := vm.stateDB.NewView(ctx, merkledb.ViewChanges{
		MapOps:       ts.ChangedKeys(),
		ConsumeBytes: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to commit genesis initialized state diff: %w", err)
	}
	if err := view.CommitToDB(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit genesis view: %w", err)
	}
	root, err := vm.stateDB.GetMerkleRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initialized genesis root: %w", err)
	}
	vm.snowCtx.Log.Info("genesis state created", zap.Stringer("root", root))
	// Create genesis block
	genesisExecutionBlk, err := chain.NewGenesisBlock(GenesisTime, root)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}
	genesisExecutedBlock := chain.NewExecutedBlock(genesisExecutionBlk, nil /* no txs in genesis => no results */, feeManager.UnitPrices(), fees.Dimensions{})
	if err := vm.chainIndex.UpdateLastAccepted(ctx, genesisExecutedBlock); err != nil {
		return nil, fmt.Errorf("failed to write genesis block: %w", err)
	}

	return &chain.OutputBlock{
		ExecutionBlock:   genesisExecutionBlk,
		View:             vm.stateDB,
		ExecutionResults: &chain.ExecutionResults{},
	}, nil
}

func (vm *VM) executeBlock(ctx context.Context, parentBlock *chain.OutputBlock, block *chain.ExecutionBlock) (*chain.OutputBlock, error) {
	panic("not implemented")
}

func (vm *VM) startNormalOp(ctx context.Context) error {
	vm.builder.Start()
	vm.snowApp.AddCloser("builder", func() error {
		vm.builder.Done()
		return nil
	})

	vm.gossiper.Start(vm.network.NewClient(txGossipHandlerID))
	vm.snowApp.AddCloser("gossiper", func() error {
		vm.gossiper.Done()
		return nil
	})

	if err := vm.network.AddHandler(
		txGossipHandlerID,
		gossiper.NewTxGossipHandler(
			vm.snowCtx.Log,
			vm.gossiper,
		),
	); err != nil {
		return fmt.Errorf("failed to add tx gossip handler: %w", err)
	}
	vm.checkActivity(ctx)
	vm.normalOp.Store(true)

	return nil
}

// applyModuleOptions configures the optional modules of the VM ie. builder/gossiper
// which are part of the VM and can be configured via options.
func (vm *VM) applyModuleOptions() error {
	if vm.options.builder {
		vm.builder = builder.NewManual(vm.snowInput.ToEngine, vm.snowCtx.Log)
	} else {
		vm.builder = builder.NewTime(vm.snowInput.ToEngine, vm.snowCtx.Log, vm.mempool, func(ctx context.Context, t int64) (int64, int64, error) {
			blk, err := vm.consensusIndex.GetPreferredBlock(ctx)
			if err != nil {
				return 0, 0, err
			}

			return blk.GetTimestamp(), vm.ruleFactory.GetRules(t).GetMinBlockGap(), nil
		})
	}

	gossipRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, gossiperNamespace)
	if err != nil {
		return fmt.Errorf("failed to register %s metrics: %w", gossiperNamespace, err)
	}
	batchedTxSerializer := &chain.BatchedTransactionSerializer{
		Parser: vm.txParser,
	}
	if vm.options.gossiper {
		vm.gossiper, err = gossiper.NewManual[*chain.Transaction](
			vm.snowCtx.Log,
			gossipRegistry,
			vm.mempool,
			batchedTxSerializer,
			vm,
			vm.config.TargetGossipDuration,
		)
		if err != nil {
			return fmt.Errorf("failed to create manual gossiper: %w", err)
		}
	} else {
		partitionTarget, err := fortification.NewTxPartition[*chain.Transaction](vm.snowCtx.Log, vm.chainState)
		if err != nil {
			return fmt.Errorf("failed to create partition target: %w", err)
		}
		partitionAssigner := gossiper.NewTargetAssigner(vm.snowCtx.NodeID, partitionTarget)
		txGossiper, err := gossiper.NewTarget[*chain.Transaction](
			vm.tracer,
			vm.snowCtx.Log,
			gossipRegistry,
			vm.mempool,
			batchedTxSerializer,
			vm,
			vm,
			vm.config.TargetGossipDuration,
			partitionAssigner,
			gossiper.DefaultTargetConfig(),
			vm.snowInput.Shutdown,
		)
		if err != nil {
			return err
		}
		vm.gossiper = txGossiper
		vm.snowApp.AddVerifiedSub(event.SubscriptionFunc[*dsmr.Block]{
			NotifyF: func(_ context.Context, b *dsmr.Block) error {
				txGossiper.BlockVerified(b.GetTimestamp())
				return nil
			},
		})
	}
	return nil
}

func (vm *VM) checkActivity(ctx context.Context) {
	vm.gossiper.Queue(ctx)
	vm.builder.Queue(ctx)
}

func (vm *VM) ParseBlock(ctx context.Context, bytes []byte) (*dsmr.Block, error) {
	return dsmr.ParseBlock(bytes)
}

func (vm *VM) BuildBlock(ctx context.Context, pChainCtx *block.Context, parent *dsmr.Block) (*dsmr.Block, *dsmr.Block, error) {
	defer vm.checkActivity(ctx)

	block, err := vm.dsmrNode.BuildBlock(ctx, pChainCtx, parent)
	if err != nil {
		return nil, nil, err
	}
	return block, block, nil
}

func (vm *VM) VerifyBlock(ctx context.Context, parent *dsmr.Block, block *dsmr.Block) (*dsmr.Block, error) {
	if err := vm.dsmrNode.VerifyBlock(ctx, parent, block); err != nil {
		return nil, err
	}
	return block, nil
}

func (vm *VM) AcceptBlock(ctx context.Context, acceptedParent *chain.OutputBlock, block *dsmr.Block) (*chain.OutputBlock, error) {
	return vm.dsmrNode.AcceptBlock(ctx, block)
}

func (vm *VM) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// Create temporary execution context
	preferredBlk, err := vm.consensusIndex.GetLastAccepted(ctx)
	if err != nil {
		vm.snowCtx.Log.Error("failed to fetch preferred block for tx submission", zap.Error(err))
		return []error{err}
	}
	view := preferredBlk.View

	validTxs := []*chain.Transaction{}
	for _, tx := range txs {
		// Avoid any sig verification or state lookup if we already have tx in mempool
		txID := tx.GetID()
		if vm.mempool.Has(ctx, txID) {
			// Don't remove from listeners, it will be removed elsewhere if not
			// included
			errs = append(errs, ErrNotAdded)
			continue
		}

		if err := vm.preExecutor.PreExecute(ctx, preferredBlk.ExecutionBlock, view, tx); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, nil)
		validTxs = append(validTxs, tx)
	}
	vm.mempool.Add(ctx, validTxs)
	vm.checkActivity(ctx)
	vm.metrics.mempoolSize.Set(float64(vm.mempool.Len(ctx)))
	vm.snowCtx.Log.Info("Submitted tx(s)", zap.Int("validTxs", len(validTxs)), zap.Int("invalidTxs", len(errs)-len(validTxs)), zap.Int("mempoolSize", vm.mempool.Len(ctx)))
	return errs
}
