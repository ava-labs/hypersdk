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

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chainindex"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/builder"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validators"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/ava-labs/hypersdk/storage"

	avatrace "github.com/ava-labs/avalanchego/trace"
	hsnow "github.com/ava-labs/hypersdk/snow"
)

const (
	blockDB             = "blockdb"
	stateDB             = "statedb"
	resultsDB           = "results"
	lastResultKey       = byte(0)
	syncerDB            = "syncerdb"
	vmDataDir           = "vm"
	hyperNamespace      = "hypervm"
	chainNamespace      = "chain"
	chainIndexNamespace = "chainindex"
	gossiperNamespace   = "gossiper"

	changeProofHandlerID = 0x0
	rangeProofHandlerID  = 0x1
	txGossipHandlerID    = 0x2
	blockFetchHandleID   = 0x3
)

var ErrNotAdded = errors.New("not added")

var (
	_ hsnow.Block = (*chain.ExecutionBlock)(nil)
	_ hsnow.Block = (*chain.OutputBlock)(nil)

	_ hsnow.Chain[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] = (*VM)(nil)
	_ hsnow.ChainIndex[*chain.ExecutionBlock]                                    = (*chainindex.ChainIndex[*chain.ExecutionBlock])(nil)
)

type VM struct {
	snowInput hsnow.ChainInput
	snowApp   *hsnow.VM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]

	proposerMonitor *validators.ProposerMonitor

	config Config

	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	genesis               genesis.Genesis
	GenesisBytes          []byte
	ruleFactory           chain.RuleFactory
	options               []Option

	chain                   *chain.Chain
	chainTimeValidityWindow *validitywindow.TimeValidityWindow[*chain.Transaction]
	SyncClient              *statesync.Client[*chain.ExecutionBlock]

	consensusIndex *hsnow.ConsensusIndex[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]
	chainStore     *chainindex.ChainIndex[*chain.ExecutionBlock]

	normalOp atomic.Bool
	builder  builder.Builder
	gossiper gossiper.Gossiper
	mempool  *mempool.Mempool[*chain.Transaction]

	vmAPIHandlerFactories []api.HandlerFactory[api.VM]
	rawStateDB            database.Database
	stateDB               merkledb.MerkleDB
	executionResultsDB    database.Database
	balanceHandler        chain.BalanceHandler
	metadataManager       chain.MetadataManager
	txParser              chain.Parser
	abi                   abi.ABI
	actionCodec           *codec.TypeParser[chain.Action]
	authCodec             *codec.TypeParser[chain.Auth]
	outputCodec           *codec.TypeParser[codec.Typed]
	authEngines           auth.Engines

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
	authEngines auth.Engines,
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
		authEngines:           authEngines,
		genesisAndRuleFactory: genesisFactory,
		options:               options,
	}, nil
}

// implements "block.ChainVM.common.VM"
func (vm *VM) Initialize(
	ctx context.Context,
	chainInput hsnow.ChainInput,
	snowApp *hsnow.VM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock],
) (hsnow.ChainIndex[*chain.ExecutionBlock], *chain.OutputBlock, *chain.OutputBlock, bool, error) {
	var (
		snowCtx      = chainInput.SnowCtx
		genesisBytes = chainInput.GenesisBytes
		upgradeBytes = chainInput.UpgradeBytes
	)
	vm.DataDir = filepath.Join(snowCtx.ChainDataDir, vmDataDir)
	vm.snowCtx = snowCtx
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

	vm.genesis, vm.ruleFactory, err = vm.genesisAndRuleFactory.Load(genesisBytes, upgradeBytes, vm.snowCtx.NetworkID, vm.snowCtx.ChainID)
	vm.GenesisBytes = genesisBytes
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

	vm.mempool = mempool.New[*chain.Transaction](vm.tracer, vm.config.MempoolSize, vm.config.MempoolSponsorSize)
	snowApp.AddAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			droppedTxs := vm.mempool.SetMinTimestamp(ctx, b.Tmstmp)
			vm.snowCtx.Log.Debug("dropping expired transactions from mempool",
				zap.Stringer("blkID", b.GetID()),
				zap.Int("numTxs", len(droppedTxs)),
			)
			return nil
		},
	})
	snowApp.AddVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.mempool.Remove(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})
	snowApp.AddRejectedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.mempool.Add(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})

	// Instantiate DBs
	pebbleConfig := pebble.NewDefaultConfig()
	stateDBRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, stateDB)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to register statedb metrics: %w", err)
	}
	vm.rawStateDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, stateDB, stateDBRegistry)
	if err != nil {
		return nil, nil, nil, false, err
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
		return nil, nil, nil, false, err
	}
	snowApp.AddCloser(stateDB, func() error {
		if err := vm.stateDB.Close(); err != nil {
			return fmt.Errorf("failed to close state db: %w", err)
		}
		if err := vm.rawStateDB.Close(); err != nil {
			return fmt.Errorf("failed to close raw state db: %w", err)
		}
		return nil
	})
	vm.executionResultsDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, resultsDB, prometheus.NewRegistry() /* throwaway metrics registry */)
	if err != nil {
		return nil, nil, nil, false, err
	}
	snowApp.AddCloser(resultsDB, func() error {
		if err := vm.executionResultsDB.Close(); err != nil {
			return fmt.Errorf("failed to close execution results db: %w", err)
		}
		return nil
	})

	// Setup worker cluster for verifying signatures
	//
	// If [parallelism] is odd, we assign the extra
	// core to signature verification.
	vm.authVerifiers = workers.NewParallel(vm.config.AuthVerificationCores, 100) // TODO: make job backlog a const
	snowApp.AddCloser("auth verifiers", func() error {
		vm.authVerifiers.Stop()
		return nil
	})

	// Set defaults
	options := &Options{}
	for _, Option := range vm.options {
		opt, err := Option.optionFunc(vm, vm.snowInput.Config.GetRawConfig(Option.Namespace))
		if err != nil {
			return nil, nil, nil, false, err
		}
		opt.apply(options)
	}
	err = vm.applyOptions(options)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to apply options : %w", err)
	}

	vm.chainTimeValidityWindow = validitywindow.NewTimeValidityWindow(vm.snowCtx.Log, vm.tracer, vm, func(timestamp int64) int64 {
		return vm.ruleFactory.GetRules(timestamp).GetValidityWindow()
	})
	chainRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, chainNamespace)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to make %q registry: %w", chainNamespace, err)
	}
	chainConfig, err := GetChainConfig(chainInput.Config)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to get chain config: %w", err)
	}
	vm.chain, err = chain.NewChain(
		vm.Tracer(),
		chainRegistry,
		vm.txParser,
		vm.Mempool(),
		vm.Logger(),
		vm.ruleFactory,
		vm.MetadataManager(),
		vm.BalanceHandler(),
		vm.AuthVerifiers(),
		vm.authEngines,
		vm.chainTimeValidityWindow,
		chainConfig,
	)
	if err != nil {
		return nil, nil, nil, false, err
	}

	if err := vm.initChainStore(); err != nil {
		return nil, nil, nil, false, err
	}

	if err := vm.initStateSync(ctx); err != nil {
		return nil, nil, nil, false, err
	}

	if err := vm.populateValidityWindow(ctx); err != nil {
		return nil, nil, nil, false, err
	}

	snowApp.AddNormalOpStarter(func(_ context.Context) error {
		if vm.SyncClient.Started() {
			return nil
		}
		return vm.startNormalOp(ctx)
	})

	for _, apiFactory := range vm.vmAPIHandlerFactories {
		api, err := apiFactory.New(vm)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("failed to initialize api: %w", err)
		}
		snowApp.AddHandler(api.Path, api.Handler)
	}

	stateReady := !vm.SyncClient.MustStateSync()
	var lastAccepted *chain.OutputBlock
	if stateReady {
		lastAccepted, err = vm.initLastAccepted(ctx)
		if err != nil {
			return nil, nil, nil, false, err
		}
	}
	return vm.chainStore, lastAccepted, lastAccepted, stateReady, nil
}

func (vm *VM) SetConsensusIndex(consensusIndex *hsnow.ConsensusIndex[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]) {
	vm.consensusIndex = consensusIndex
}

func (vm *VM) initChainStore() error {
	blockDBRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, blockDB)
	if err != nil {
		return fmt.Errorf("failed to register %s metrics: %w", blockDB, err)
	}
	pebbleConfig := pebble.NewDefaultConfig()
	chainStoreDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, blockDB, blockDBRegistry)
	if err != nil {
		return fmt.Errorf("failed to create chain index database: %w", err)
	}
	vm.snowApp.AddCloser(chainIndexNamespace, chainStoreDB.Close)
	config, err := GetChainIndexConfig(vm.snowInput.Config)
	if err != nil {
		return fmt.Errorf("failed to create chain index config: %w", err)
	}
	vm.chainStore, err = chainindex.New[*chain.ExecutionBlock](vm.snowCtx.Log, blockDBRegistry, config, vm.chain, chainStoreDB)
	if err != nil {
		return fmt.Errorf("failed to create chain index: %w", err)
	}
	return nil
}

func (vm *VM) initLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	lastAcceptedHeight, err := vm.chainStore.GetLastAcceptedHeight(ctx)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to load genesis block: %w", err)
	}
	if err == database.ErrNotFound {
		return vm.initGenesisAsLastAccepted(ctx)
	}
	if lastAcceptedHeight == 0 {
		blk, err := vm.chainStore.GetBlockByHeight(ctx, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch genesis block: %w", err)
		}
		return &chain.OutputBlock{
			ExecutionBlock:   blk,
			View:             vm.stateDB,
			ExecutionResults: &chain.ExecutionResults{},
		}, nil
	}

	// If the chain index is initialized, return the output block that matches with the latest
	// state.
	return vm.extractLatestOutputBlock(ctx)
}

func (vm *VM) extractStateHeight() (uint64, error) {
	heightBytes, err := vm.stateDB.Get(chain.HeightKey(vm.metadataManager.HeightPrefix()))
	if err != nil {
		return 0, fmt.Errorf("failed to get state height: %w", err)
	}
	stateHeight, err := database.ParseUInt64(heightBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to parse state height: %w", err)
	}
	return stateHeight, nil
}

func (vm *VM) extractLatestOutputBlock(ctx context.Context) (*chain.OutputBlock, error) {
	stateHeight, err := vm.extractStateHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get state hegiht for latest output block: %w", err)
	}
	lastIndexedHeight, err := vm.chainStore.GetLastAcceptedHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get last accepted height: %w", err)
	}
	if lastIndexedHeight != stateHeight && lastIndexedHeight != stateHeight+1 {
		return nil, fmt.Errorf("cannot extract latest output block from invalid state with last indexed height %d and state height %d", lastIndexedHeight, stateHeight)
	}

	// If the heights match exactly, we must have stored the last execution results
	if lastIndexedHeight == stateHeight {
		resultBytes, err := vm.executionResultsDB.Get([]byte{lastResultKey})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch last execution results: %w", err)
		}
		if len(resultBytes) < consts.Uint64Len {
			return nil, fmt.Errorf("invalid execution results length: %d", len(resultBytes))
		}
		executionResultsHeight, err := database.ParseUInt64(resultBytes[len(resultBytes)-consts.Uint64Len:])
		if err != nil {
			return nil, fmt.Errorf("failed to parse execution results height: %w", err)
		}
		if executionResultsHeight != stateHeight {
			return nil, fmt.Errorf("execution results height %d does not match state height %d", executionResultsHeight, stateHeight)
		}
		blk, err := vm.chainStore.GetBlockByHeight(ctx, stateHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at latest state height %d: %w", stateHeight, err)
		}
		executionResults, err := chain.ParseExecutionResults(resultBytes[:len(resultBytes)-consts.Uint64Len])
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal execution results for last accepted block: %w", err)
		}
		return &chain.OutputBlock{
			ExecutionBlock:   blk,
			View:             vm.stateDB,
			ExecutionResults: executionResults,
		}, nil
	}

	// The last indexedHeight must be stateHeight+1, so we can execute the last block to populate
	// execution results
	blk, err := vm.chainStore.GetBlockByHeight(ctx, stateHeight+1)
	if err != nil {
		return nil, fmt.Errorf("failed to get block at latest state height %d: %w", stateHeight, err)
	}
	outputBlock, err := vm.chain.Execute(ctx, vm.stateDB, blk, false)
	if err != nil {
		return nil, fmt.Errorf("failed to execute block at latest state height %d: %w", stateHeight, err)
	}
	if _, err := vm.AcceptBlock(ctx, nil, outputBlock); err != nil {
		return nil, err
	}
	return outputBlock, nil
}

func (vm *VM) initGenesisAsLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	genesisExecutionBlk, genesisView, err := chain.NewGenesisCommit(
		ctx,
		vm.stateDB,
		vm.genesis,
		vm.metadataManager,
		vm.balanceHandler,
		vm.ruleFactory,
		vm.tracer,
		vm.snowCtx.Log,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis state diff: %w", err)
	}

	vm.snowCtx.Log.Info(
		"genesis state created",
		zap.Stringer("root", genesisExecutionBlk.GetStateRoot()),
	)

	if err := genesisView.CommitToDB(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit genesis view: %w", err)
	}
	if err := vm.chainStore.UpdateLastAccepted(ctx, genesisExecutionBlk); err != nil {
		return nil, fmt.Errorf("failed to write genesis block: %w", err)
	}

	return &chain.OutputBlock{
		ExecutionBlock:   genesisExecutionBlk,
		View:             vm.stateDB,
		ExecutionResults: &chain.ExecutionResults{},
	}, nil
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

func (vm *VM) applyOptions(o *Options) error {
	blockSubs := make([]event.Subscription[*chain.ExecutedBlock], len(o.blockSubscriptionFactories))
	for i, factory := range o.blockSubscriptionFactories {
		sub, err := factory.New()
		if err != nil {
			return err
		}
		blockSubs[i] = sub
	}
	executedBlockSub := event.Aggregate(blockSubs...)
	outputBlockSub := event.Map(func(b *chain.OutputBlock) *chain.ExecutedBlock {
		return &chain.ExecutedBlock{
			Block:            b.StatelessBlock,
			ExecutionResults: b.ExecutionResults,
		}
	}, executedBlockSub)
	vm.snowApp.AddAcceptedSub(outputBlockSub)
	vm.vmAPIHandlerFactories = o.vmAPIHandlerFactories
	if o.builder {
		vm.builder = builder.NewManual(vm.snowInput.ToEngine, vm.snowCtx.Log)
	} else {
		vm.builder = builder.NewTime(vm.snowInput.ToEngine, vm.snowCtx.Log, vm.mempool, func(ctx context.Context, t int64) (int64, int64, error) {
			blk, err := vm.consensusIndex.GetPreferredBlock(ctx)
			if err != nil {
				return 0, 0, err
			}
			return blk.Tmstmp, vm.ruleFactory.GetRules(t).GetMinBlockGap(), nil
		})
	}

	gossipRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, gossiperNamespace)
	if err != nil {
		return fmt.Errorf("failed to register %s metrics: %w", gossiperNamespace, err)
	}
	batchedTxSerializer := &chain.BatchedTransactionSerializer{
		Parser: vm.txParser,
	}
	if o.gossiper {
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
		txGossiper, err := gossiper.NewTarget[*chain.Transaction](
			vm.tracer,
			vm.snowCtx.Log,
			gossipRegistry,
			vm.mempool,
			batchedTxSerializer,
			vm,
			vm,
			vm.config.TargetGossipDuration,
			&gossiper.TargetProposers[*chain.Transaction]{
				Validators: vm,
				Config:     gossiper.DefaultTargetProposerConfig(),
			},
			gossiper.DefaultTargetConfig(),
			vm.snowInput.Shutdown,
		)
		if err != nil {
			return err
		}
		vm.gossiper = txGossiper
		vm.snowApp.AddVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
			NotifyF: func(_ context.Context, b *chain.OutputBlock) error {
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

func (vm *VM) ParseBlock(ctx context.Context, source []byte) (*chain.ExecutionBlock, error) {
	return vm.chain.ParseBlock(ctx, source)
}

func (vm *VM) BuildBlock(ctx context.Context, pChainCtx *block.Context, parent *chain.OutputBlock) (*chain.ExecutionBlock, *chain.OutputBlock, error) {
	defer vm.checkActivity(ctx)

	return vm.chain.BuildBlock(ctx, pChainCtx, parent)
}

func (vm *VM) VerifyBlock(ctx context.Context, parent *chain.OutputBlock, block *chain.ExecutionBlock) (*chain.OutputBlock, error) {
	return vm.chain.Execute(ctx, parent.View, block, vm.normalOp.Load())
}

func (vm *VM) AcceptBlock(ctx context.Context, _ *chain.OutputBlock, block *chain.OutputBlock) (*chain.OutputBlock, error) {
	resultBytes := block.ExecutionResults.Marshal()
	resultBytes = binary.BigEndian.AppendUint64(resultBytes, block.Hght)
	if err := vm.executionResultsDB.Put([]byte{lastResultKey}, resultBytes); err != nil {
		return nil, fmt.Errorf("failed to write execution results: %w", err)
	}

	if err := vm.chain.AcceptBlock(ctx, block); err != nil {
		return nil, fmt.Errorf("failed to accept block %s: %w", block, err)
	}
	return block, nil
}

func (vm *VM) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// Create temporary execution context
	preferredBlk, err := vm.consensusIndex.GetPreferredBlock(ctx)
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

		if err := vm.chain.PreExecute(ctx, preferredBlk.ExecutionBlock, view, tx); err != nil {
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

// populateValidityWindow populates the VM's time validity window on startup,
// ensuring it contains recent transactions even if state sync is skipped (e.g., due to restart).
// This is necessary because a node might restart with only a few blocks behind (or slightly ahead)
// of the network, and thus opt not to trigger state sync. Without backfilling, the node's validity window
// may be incomplete, causing the node to accept a duplicate transaction that the network already processed.

// When Initialize is called, vm.consensusIndex is nil—it is set later via SetConsensusIndex.
// Therefore, we must use the chainStore (which reads blocks from disk) to backfill the validity window.
// This prepopulation ensures the validity window is complete, even if state sync is skipped.
func (vm *VM) populateValidityWindow(ctx context.Context) error {
	lastAcceptedBlkHeight, err := vm.chainStore.GetLastAcceptedHeight(ctx)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		return err
	}
	lastAcceptedBlock, err := vm.chainStore.GetBlockByHeight(ctx, lastAcceptedBlkHeight)
	if err != nil {
		return err
	}

	vm.chainTimeValidityWindow.PopulateValidityWindow(ctx, lastAcceptedBlock)
	return nil
}
