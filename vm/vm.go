// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chainstore"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/builder"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validators"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/lifecycle"
	hsnow "github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/storage"

	avatrace "github.com/ava-labs/avalanchego/trace"
)

const (
	blockDB   = "blockdb"
	stateDB   = "statedb"
	syncerDB  = "syncerdb"
	vmDataDir = "vm"

	MaxAcceptorSize        = 256
	MinAcceptedBlockWindow = 1024

	changeProofHandlerID = 0x0
	rangeProofHandlerID  = 0x1
	txGossipHandlerID    = 0x2
)

type VM struct {
	snowInput   hsnow.ChainInput
	snowApp *hsnow.Application[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]

	proposerMonitor *validators.ProposerMonitor

	config Config

	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	genesis               genesis.Genesis
	GenesisBytes          []byte
	ruleFactory           chain.RuleFactory
	options               []Option

	chain                   *chain.Chain
	chainTimeValidityWindow chain.ValidityWindow
	syncer                  *validitywindow.Syncer[*chain.Transaction]

	chainIndex hsnow.ChainIndex[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]
	chainStore *chainstore.ChainStore[*chain.ExecutionBlock]

	builder  builder.Builder
	gossiper gossiper.Gossiper
	mempool  *mempool.Mempool[*chain.Transaction]

	vmAPIHandlerFactories []api.HandlerFactory[api.VM]
	rawStateDB            database.Database
	stateDB               merkledb.MerkleDB
	vmDB                  database.Database
	balanceHandler        chain.BalanceHandler
	metadataManager       chain.MetadataManager
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

	return &VM{
		balanceHandler:        balanceHandler,
		metadataManager:       metadataManager,
		config:                NewConfig(),
		actionCodec:           actionCodec,
		authCodec:             authCodec,
		outputCodec:           outputCodec,
		authEngine:            authEngine,
		genesisAndRuleFactory: genesisFactory,
		options:               options,
	}, nil
}

// implements "block.ChainVM.common.VM"
func (vm *VM) Initialize(
	ctx context.Context,
	chainInput hsnow.ChainInput,
	chainIndex hsnow.ChainIndex[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock],
	snowApp *hsnow.Application[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock],
) (hsnow.BlockChainIndex[*chain.ExecutionBlock], *chain.OutputBlock, *chain.OutputBlock, bool, error) {
	var (
		snowCtx      = chainInput.SnowCtx
		genesisBytes = chainInput.GenesisBytes
		upgradeBytes = chainInput.UpgradeBytes
		configBytes  = chainInput.ConfigBytes
	)
	vm.DataDir = filepath.Join(snowCtx.ChainDataDir, vmDataDir)
	vm.snowCtx = snowCtx
	vm.snowInput = chainInput
	// TODO: cleanup metrics registration
	defaultRegistry, metrics, err := newMetrics()
	if err != nil {
		return nil, nil, nil, false, err
	}
	if err := vm.snowCtx.Metrics.Register("hypersdk", defaultRegistry); err != nil {
		return nil, nil, nil, false, err
	}
	vm.metrics = metrics
	vm.proposerMonitor = validators.NewProposerMonitor(vm, vm.snowCtx)

	vm.network = snowApp.Network

	blockDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("blockdb", blockDBRegistry); err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to register blockdb metrics: %w", err)
	}
	pebbleConfig := pebble.NewDefaultConfig()
	vm.vmDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, blockDB, blockDBRegistry)
	if err != nil {
		return nil, nil, nil, false, err
	}
	snowApp.WithCloser(func() error {
		if err := vm.vmDB.Close(); err != nil {
			return fmt.Errorf("failed to close vm db: %w", err)
		}
		return nil
	})

	rawStateDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("rawstatedb", rawStateDBRegistry); err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to register rawstatedb metrics: %w", err)
	}
	vm.rawStateDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, stateDB, rawStateDBRegistry)
	if err != nil {
		return nil, nil, nil, false, err
	}
	snowApp.WithCloser(func() error {
		if err := vm.rawStateDB.Close(); err != nil {
			return fmt.Errorf("failed to close raw state db: %w", err)
		}
		return nil
	})

	vm.genesis, vm.ruleFactory, err = vm.genesisAndRuleFactory.Load(genesisBytes, upgradeBytes, vm.snowCtx.NetworkID, vm.snowCtx.ChainID)
	vm.GenesisBytes = genesisBytes
	if err != nil {
		return nil, nil, nil, false, err
	}

	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &vm.config); err != nil {
			return nil, nil, nil, false, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}
	snowCtx.Log.Info("initialized hypersdk config", zap.Any("config", vm.config))

	vm.tracer = chainInput.Context.Tracer()
	ctx, span := vm.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	vm.mempool = mempool.New[*chain.Transaction](vm.tracer, vm.config.MempoolSize, vm.config.MempoolSponsorSize)
	snowApp.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			droppedTxs := vm.mempool.SetMinTimestamp(ctx, b.Tmstmp)
			vm.snowCtx.Log.Debug("dropping expired transactions from mempool",
				zap.Stringer("blkID", b.ID()),
				zap.Int("numTxs", len(droppedTxs)),
			)
			return nil
		},
	})
	snowApp.WithVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.mempool.Remove(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})
	snowApp.WithRejectedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.mempool.Add(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})

	// Instantiate DBs
	merkleRegistry := prometheus.NewRegistry()
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
		Reg:                         merkleRegistry,
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.tracer,
	})
	if err != nil {
		return nil, nil, nil, false, err
	}
	snowApp.WithCloser(func() error {
		if err := vm.stateDB.Close(); err != nil {
			return fmt.Errorf("failed to close state db: %w", err)
		}
		return nil
	})
	if err := vm.snowCtx.Metrics.Register("state", merkleRegistry); err != nil {
		return nil, nil, nil, false, err
	}

	// Setup worker cluster for verifying signatures
	//
	// If [parallelism] is odd, we assign the extra
	// core to signature verification.
	vm.authVerifiers = workers.NewParallel(vm.config.AuthVerificationCores, 100) // TODO: make job backlog a const
	snowApp.WithCloser(func() error {
		vm.authVerifiers.Stop()
		return nil
	})

	acceptorSize := vm.config.AcceptorSize
	if acceptorSize > MaxAcceptorSize {
		return nil, nil, nil, false, fmt.Errorf("AcceptorSize (%d) must be <= MaxAcceptorSize (%d)", acceptorSize, MaxAcceptorSize)
	}
	acceptedBlockWindow := vm.config.AcceptedBlockWindow
	if acceptedBlockWindow < MinAcceptedBlockWindow {
		return nil, nil, nil, false, fmt.Errorf("AcceptedBlockWindow (%d) must be >= to MinAcceptedBlockWindow (%d)", acceptedBlockWindow, MinAcceptedBlockWindow)
	}

	// Set defaults
	options := &Options{}
	for _, Option := range vm.options {
		config := vm.config.ServiceConfig[Option.Namespace]
		opt, err := Option.optionFunc(vm, config)
		if err != nil {
			return nil, nil, nil, false, err
		}
		opt.apply(options)
	}
	err = vm.applyOptions(options)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to apply options : %w", err)
	}

	vm.chainTimeValidityWindow = validitywindow.NewTimeValidityWindow(vm.snowCtx.Log, vm.tracer, vm)
	snowApp.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.chainTimeValidityWindow.Accept(b)
			return nil

		},
	})
	registerer := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("chain", registerer); err != nil {
		return nil, nil, nil, false, err
	}
	vm.chain, err = chain.NewChain(
		vm.Tracer(),
		registerer,
		vm,
		vm.Mempool(),
		vm.Logger(),
		vm.ruleFactory,
		vm.MetadataManager(),
		vm.BalanceHandler(),
		vm.AuthVerifiers(),
		vm,
		vm.chainTimeValidityWindow,
		vm.config.ChainConfig,
	)
	if err != nil {
		return nil, nil, nil, false, err
	}
	vm.syncer = validitywindow.NewSyncer(vm, vm.chainTimeValidityWindow, func(time int64) int64 {
		return vm.ruleFactory.GetRules(time).GetValidityWindow()
	})
	validityWindowReady := lifecycle.NewChanReady()
	// TODO: combine time validity window and state syncer to call FinishStateSync correclty
	snowApp.WithPreReadyAcceptedSub(event.SubscriptionFunc[*chain.ExecutionBlock]{
		NotifyF: func(ctx context.Context, b *chain.ExecutionBlock) error {
			vm.metrics.txsAccepted.Add(float64(len(b.StatelessBlock.Txs)))
			seenValidityWindow, err := vm.syncer.Accept(ctx, b)
			if err != nil {
				return fmt.Errorf("syncer failed to accept block: %w", err)
			}
			if seenValidityWindow {
				validityWindowReady.MarkReady()
			}

			return nil
		},
	})
	snowApp.WithVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.metrics.txsVerified.Add(float64(len(b.StatelessBlock.Txs)))
			return nil
		},
	})

	if err := vm.initChainStore(ctx); err != nil {
		return nil, nil, nil, false, err
	}
	lastAccepted, err := vm.initLastAccepted(ctx)
	if err != nil {
		return nil, nil, nil, false, err
	}

	// Initialize the syncer with the last accepted block
	vm.syncer.Accept(ctx, lastAccepted.ExecutionBlock)

	syncerDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register(syncerDB, syncerDBRegistry); err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to register syncerdb metrics: %w", err)
	}
	syncerDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, syncerDB, syncerDBRegistry)
	if err != nil {
		return nil, nil, nil, false, err
	}
	snowApp.WithCloser(func() error {
		if err := syncerDB.Close(); err != nil {
			return fmt.Errorf("failed to close syncer db: %w", err)
		}
		return nil
	})
	if err := snowApp.WithStateSyncer(
		syncerDB,
		vm.stateDB,
		rangeProofHandlerID,
		changeProofHandlerID,
		vm.genesis.GetStateBranchFactor(),
	); err != nil {
		return nil, nil, nil, false, err
	}

	if err := vm.network.AddHandler(
		txGossipHandlerID,
		gossiper.NewTxGossipHandler(
			vm.snowApp.Ready,
			vm.snowCtx.Log,
			vm.gossiper,
		),
	); err != nil {
		return nil, nil, nil, false, err
	}

	// Startup block builder and gossiper
	go vm.builder.Run()
	go vm.gossiper.Run(vm.network.NewClient(txGossipHandlerID))

	for _, apiFactory := range vm.vmAPIHandlerFactories {
		api, err := apiFactory.New(vm)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("failed to initialize api: %w", err)
		}
		snowApp.WithHandler(api.Path, api.Handler)
	}

	// TODO: return false if we are mid state sync
	return vm.chainStore, lastAccepted, lastAccepted, true, nil
}

func (vm *VM) initChainStore(ctx context.Context) error {
	blockDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("blockdb", blockDBRegistry); err != nil {
		return fmt.Errorf("failed to register blockdb metrics: %w", err)
	}
	pebbleConfig := pebble.NewDefaultConfig()
	chainStoreDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, blockDB, blockDBRegistry)
	if err != nil {
		return fmt.Errorf("failed to create chain store database: %w", err)
	}
	vm.snowApp.WithCloser(chainStoreDB.Close)
	vm.chainStore, err = chainstore.New[*chain.ExecutionBlock](vm.snowInput.Context, vm.chain, chainStoreDB)
	return nil
}

func (vm *VM) initLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	_, err := vm.chainStore.GetLastAcceptedHeight(ctx)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to load genesis block: %w", err)
	}
	if err == database.ErrNotFound {
		return vm.initGenesisAsLastAccepted(ctx)
	}

	// If the chain store is initialized, return the output block that matches with the latest
	// state.
	heightBytes, err := vm.stateDB.Get(chain.HeightKey(vm.metadataManager.HeightPrefix()))
	if err != nil {
		return nil, err
	}
	stateHeight, err := database.ParseUInt64(heightBytes)
	if err != nil {
		return nil, err
	}
	blk, err := vm.chainStore.GetBlockByHeight(ctx, stateHeight)
	if err != nil {
		return nil, err
	}
	return &chain.OutputBlock{
		ExecutionBlock: blk,
		View:           MerkleDBWithNoopCommit{vm.stateDB},
		// XXX: we don't guarantee the last accepted block ExecutionResults to be populated
		// on startup.
		ExecutionResults: chain.ExecutionResults{},
	}, nil
}

func (vm *VM) initGenesisAsLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	sps := state.NewSimpleMutable(vm.stateDB)
	if err := vm.genesis.InitializeState(ctx, vm.tracer, sps, vm.balanceHandler); err != nil {
		return nil, fmt.Errorf("failed to initialize genesis state: %w", err)
	}
	if err := sps.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit genesis state: %w", err)
	}
	root, err := vm.stateDB.GetMerkleRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initialized genesis root: %w", err)
	}
	vm.snowCtx.Log.Info("genesis state created", zap.Stringer("root", root))

	// Create genesis block
	genesisExecutionBlk, err := chain.NewGenesisBlock(root)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}
	// Set executed block, since we will never execute the genesis block

	// Update chain metadata
	sps = state.NewSimpleMutable(vm.stateDB)
	if err := sps.Insert(ctx, chain.HeightKey(vm.metadataManager.HeightPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, fmt.Errorf("failed to set genesis height: %w", err)
	}
	if err := sps.Insert(ctx, chain.TimestampKey(vm.metadataManager.TimestampPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, fmt.Errorf("failed to set genesis timestamp: %w", err)
	}
	genesisRules := vm.ruleFactory.GetRules(0)
	feeManager := internalfees.NewManager(nil)
	minUnitPrice := genesisRules.GetMinUnitPrice()
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		feeManager.SetUnitPrice(i, minUnitPrice[i])
		vm.snowCtx.Log.Info("set genesis unit price", zap.Int("dimension", int(i)), zap.Uint64("price", feeManager.UnitPrice(i)))
	}
	if err := sps.Insert(ctx, chain.FeeKey(vm.metadataManager.FeePrefix()), feeManager.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to set genesis fee manager: %w", err)
	}

	// Commit genesis block post-execution state and compute root
	if err := sps.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit genesis state: %w", err)
	}
	if _, err := vm.stateDB.GetMerkleRoot(ctx); err != nil {
		return nil, fmt.Errorf("failed to get genesis root: %w", err)
	}
	if err := vm.chainStore.Accept(ctx, genesisExecutionBlk); err != nil {
		return nil, fmt.Errorf("failed to write genesis block: %w", err)
	}

	return &chain.OutputBlock{
		ExecutionBlock:   genesisExecutionBlk,
		View:             MerkleDBWithNoopCommit{vm.stateDB},
		ExecutionResults: chain.ExecutionResults{},
	}, nil
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
	vm.snowApp.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			executedBlock := &chain.ExecutedBlock{
				Block:            b.StatelessBlock,
				ExecutionResults: b.ExecutionResults,
			}
			errs := make([]error, len(blockSubs))
			for i, sub := range blockSubs {
				errs[i] = sub.Notify(ctx, executedBlock)
			}
			return errors.Join(errs...)
		},
		Closer: func() error {
			errs := make([]error, len(blockSubs))
			for i, sub := range blockSubs {
				errs[i] = sub.Close()
			}
			return errors.Join(errs...)
		},
	})
	vm.vmAPIHandlerFactories = o.vmAPIHandlerFactories
	if o.builder {
		vm.builder = builder.NewManual(vm.snowInput.ToEngine, vm.snowCtx.Log)
	} else {
		vm.builder = builder.NewTime(vm.snowInput.ToEngine, vm.snowCtx.Log, vm.mempool, func(ctx context.Context, t int64) (int64, int64, error) {
			blk, err := vm.chainIndex.GetPreferredBlock(ctx)
			if err != nil {
				return 0, 0, err
			}
			return blk.Tmstmp, vm.ruleFactory.GetRules(t).GetMinBlockGap(), nil
		})
	}
	vm.snowApp.WithCloser(func() error {
		vm.builder.Done()
		return nil
	})

	gossipRegistry := prometheus.NewRegistry()
	err := vm.snowCtx.Metrics.Register("gossiper", gossipRegistry)
	if err != nil {
		return fmt.Errorf("failed to register gossiper metrics: %w", err)
	}
	if o.gossiper {
		vm.gossiper, err = gossiper.NewManual[*chain.Transaction](
			vm.snowCtx.Log,
			gossipRegistry,
			vm.mempool,
			&chain.TxSerializer{
				ActionRegistry: vm.actionCodec,
				AuthRegistry:   vm.authCodec,
			},
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
			&chain.TxSerializer{
				ActionRegistry: vm.actionCodec,
				AuthRegistry:   vm.authCodec,
			},
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
		vm.snowApp.WithVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
			NotifyF: func(_ context.Context, b *chain.OutputBlock) error {
				txGossiper.BlockVerified(b.Timestamp())
				return nil
			},
		})
	}
	vm.snowApp.WithCloser(func() error {
		vm.gossiper.Done()
		return nil
	})
	return nil
}

func (vm *VM) checkActivity(ctx context.Context) {
	vm.gossiper.Queue(ctx)
	vm.builder.Queue(ctx)
}

func (vm *VM) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	return vm.stateDB.GetValues(ctx, keys)
}

func (vm *VM) ParseBlock(ctx context.Context, source []byte) (*chain.ExecutionBlock, error) {
	return vm.chain.ParseBlock(ctx, source)
}

func (vm *VM) BuildBlock(ctx context.Context, parent *chain.OutputBlock) (*chain.ExecutionBlock, *chain.OutputBlock, error) {
	return vm.chain.BuildBlock(ctx, parent)
}

func (vm *VM) Execute(ctx context.Context, parent *chain.OutputBlock, block *chain.ExecutionBlock) (*chain.OutputBlock, error) {
	return vm.chain.Execute(ctx, parent, block)
}

func (vm *VM) AcceptBlock(ctx context.Context, block *chain.OutputBlock) (*chain.OutputBlock, error) {
	if err := block.View.CommitToDB(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit state for block %s: %w", block, err)
	}
	return block, nil
}

func (vm *VM) GetBlock(ctx context.Context, blkID ids.ID) (*chain.ExecutionBlock, error) {
	return vm.chainStore.GetBlock(ctx, blkID)
}

func (vm *VM) GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error) {
	return vm.chainStore.GetBlockIDAtHeight(ctx, height)
}

func (vm *VM) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// Create temporary execution context
	preferredBlk, err := vm.chainIndex.GetPreferredBlock(ctx)
	if err != nil {
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

		if err := vm.chain.PreExecute(ctx, preferredBlk.ExecutionBlock, view, tx, vm.config.VerifyAuth); err != nil {
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
