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
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
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
	hsnow "github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/storage"
	"github.com/ava-labs/hypersdk/utils"

	avatrace "github.com/ava-labs/avalanchego/trace"
	avautils "github.com/ava-labs/avalanchego/utils"
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
	DataDir string
	v       *version.Semantic

	snowCtx         *snow.Context
	pkBytes         []byte
	proposerMonitor *validators.ProposerMonitor

	config Config

	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	genesis               genesis.Genesis
	GenesisBytes          []byte
	ruleFactory           chain.RuleFactory
	options               []Option

	snowOptions *hsnow.Options[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]

	chain                   *chain.Chain
	chainTimeValidityWindow chain.ValidityWindow
	syncer                  *validitywindow.Syncer[*chain.Transaction]
	seenValidityWindowOnce  sync.Once
	seenValidityWindow      chan struct{}

	chainIndex hsnow.ChainIndex[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]
	chainStore *chainstore.ChainStore

	builder  builder.Builder
	gossiper gossiper.Gossiper

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
	network               *p2p.Network

	tracer  avatrace.Tracer
	mempool *mempool.Mempool[*chain.Transaction]

	// authVerifiers are used to verify signatures in parallel
	// with limited parallelism
	authVerifiers workers.Workers

	bootstrapped avautils.Atomic[bool]
	toEngine     chan<- common.Message

	metrics *Metrics

	ready chan struct{}
	stop  chan struct{}
}

func New(
	v *version.Semantic,
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
		v:                     v,
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
	snowOptions *hsnow.Options[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock],
) (*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock, error) {
	var (
		snowCtx      = chainInput.SnowCtx
		genesisBytes = chainInput.GenesisBytes
		upgradeBytes = chainInput.UpgradeBytes
		configBytes  = chainInput.ConfigBytes
		toEngine     = chainInput.ToEngine
	)
	vm.DataDir = filepath.Join(snowCtx.ChainDataDir, vmDataDir)
	vm.snowCtx = snowCtx
	// Init channels before initializing other structs
	vm.toEngine = toEngine
	vm.pkBytes = bls.PublicKeyToCompressedBytes(vm.snowCtx.PublicKey)
	vm.seenValidityWindow = make(chan struct{})
	vm.ready = make(chan struct{})
	vm.stop = make(chan struct{})
	// TODO: cleanup metrics registration
	defaultRegistry, metrics, err := newMetrics()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := vm.snowCtx.Metrics.Register("hypersdk", defaultRegistry); err != nil {
		return nil, nil, nil, err
	}
	vm.metrics = metrics
	vm.proposerMonitor = validators.NewProposerMonitor(vm, vm.snowCtx)

	vm.network = snowOptions.Network

	blockDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("blockdb", blockDBRegistry); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to register blockdb metrics: %w", err)
	}
	pebbleConfig := pebble.NewDefaultConfig()
	vm.vmDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, blockDB, blockDBRegistry)
	if err != nil {
		return nil, nil, nil, err
	}
	snowOptions.WithCloser(func() error {
		if err := vm.vmDB.Close(); err != nil {
			return fmt.Errorf("failed to close vm db: %w", err)
		}
		return nil
	})

	rawStateDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("rawstatedb", rawStateDBRegistry); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to register rawstatedb metrics: %w", err)
	}
	vm.rawStateDB, err = storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, stateDB, rawStateDBRegistry)
	if err != nil {
		return nil, nil, nil, err
	}
	snowOptions.WithCloser(func() error {
		if err := vm.rawStateDB.Close(); err != nil {
			return fmt.Errorf("failed to close raw state db: %w", err)
		}
		return nil
	})

	vm.genesis, vm.ruleFactory, err = vm.genesisAndRuleFactory.Load(genesisBytes, upgradeBytes, vm.snowCtx.NetworkID, vm.snowCtx.ChainID)
	vm.GenesisBytes = genesisBytes
	if err != nil {
		return nil, nil, nil, err
	}

	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &vm.config); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}
	snowCtx.Log.Info("initialized hypersdk config", zap.Any("config", vm.config))

	vm.tracer = chainInput.Tracer
	ctx, span := vm.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	// Set defaults
	vm.mempool = mempool.New[*chain.Transaction](vm.tracer, vm.config.MempoolSize, vm.config.MempoolSponsorSize)
	snowOptions.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			droppedTxs := vm.mempool.SetMinTimestamp(ctx, b.Tmstmp)
			vm.snowCtx.Log.Debug("dropping expired transactions from mempool",
				zap.Stringer("blkID", b.ID()),
				zap.Int("numTxs", len(droppedTxs)),
			)
			return nil
		},
	})
	snowOptions.WithVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			vm.mempool.Remove(ctx, b.StatelessBlock.Txs)
			return nil
		},
	})
	snowOptions.WithRejectedSub(event.SubscriptionFunc[*chain.OutputBlock]{
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
		return nil, nil, nil, err
	}
	snowOptions.WithCloser(func() error {
		if err := vm.stateDB.Close(); err != nil {
			return fmt.Errorf("failed to close state db: %w", err)
		}
		return nil
	})
	if err := vm.snowCtx.Metrics.Register("state", merkleRegistry); err != nil {
		return nil, nil, nil, err
	}

	// Setup worker cluster for verifying signatures
	//
	// If [parallelism] is odd, we assign the extra
	// core to signature verification.
	vm.authVerifiers = workers.NewParallel(vm.config.AuthVerificationCores, 100) // TODO: make job backlog a const
	snowOptions.WithCloser(func() error {
		vm.authVerifiers.Stop()
		return nil
	})

	acceptorSize := vm.config.AcceptorSize
	if acceptorSize > MaxAcceptorSize {
		return nil, nil, nil, fmt.Errorf("AcceptorSize (%d) must be <= MaxAcceptorSize (%d)", acceptorSize, MaxAcceptorSize)
	}
	acceptedBlockWindow := vm.config.AcceptedBlockWindow
	if acceptedBlockWindow < MinAcceptedBlockWindow {
		return nil, nil, nil, fmt.Errorf("AcceptedBlockWindow (%d) must be >= to MinAcceptedBlockWindow (%d)", acceptedBlockWindow, MinAcceptedBlockWindow)
	}

	// Set defaults
	options := &Options{}
	for _, Option := range vm.options {
		config := vm.config.ServiceConfig[Option.Namespace]
		opt, err := Option.optionFunc(vm, config)
		if err != nil {
			return nil, nil, nil, err
		}
		opt.apply(options)
	}
	err = vm.applyOptions(options)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to apply options : %w", err)
	}

	vm.chainTimeValidityWindow = validitywindow.NewTimeValidityWindow(vm.snowCtx.Log, vm.tracer, vm)
	registerer := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("chain", registerer); err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}
	vm.syncer = validitywindow.NewSyncer(vm, vm.chainTimeValidityWindow, func(time int64) int64 {
		return vm.ruleFactory.GetRules(time).GetValidityWindow()
	})
	snowOptions.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
		NotifyF: func(ctx context.Context, b *chain.OutputBlock) error {
			seenValidityWindow, err := vm.syncer.Accept(ctx, b.ExecutionBlock)
			if err != nil {
				return fmt.Errorf("syncer failed to accept block: %w", err)
			}
			if seenValidityWindow {
				vm.seenValidityWindowOnce.Do(func() {
					close(vm.seenValidityWindow)
				})
			}
			return nil
		},
	})

	if err := vm.initChainStore(ctx); err != nil {
		return nil, nil, nil, err
	}
	lastAccepted, err := vm.initLastAccepted(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// accept the last block in order to initialize the internal lastBlockHeight
	vm.chainTimeValidityWindow.Accept(lastAccepted.ExecutionBlock)

	syncerDBRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register(syncerDB, syncerDBRegistry); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to register syncerdb metrics: %w", err)
	}
	syncerDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, syncerDB, syncerDBRegistry)
	if err != nil {
		return nil, nil, nil, err
	}
	snowOptions.WithCloser(func() error {
		if err := syncerDB.Close(); err != nil {
			return fmt.Errorf("failed to close syncer db: %w", err)
		}
		return nil
	})
	if err := snowOptions.WithStateSyncer(
		syncerDB,
		vm.stateDB,
		rangeProofHandlerID,
		changeProofHandlerID,
		vm.genesis.GetStateBranchFactor(),
	); err != nil {
		return nil, nil, nil, err
	}

	if err := vm.network.AddHandler(
		txGossipHandlerID,
		gossiper.NewTxGossipHandler(
			vm,
			vm.snowCtx.Log,
			vm.gossiper,
		),
	); err != nil {
		return nil, nil, nil, err
	}

	// Startup block builder and gossiper
	go vm.builder.Run()
	go vm.gossiper.Run(vm.network.NewClient(txGossipHandlerID))

	for _, apiFactory := range vm.vmAPIHandlerFactories {
		api, err := apiFactory.New(vm)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to initialize api: %w", err)
		}
		snowOptions.WithHandler(api.Path, api.Handler)
	}

	return lastAccepted.ExecutionBlock, lastAccepted, lastAccepted, nil
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
	vm.snowOptions.WithCloser(func() error {
		return chainStoreDB.Close()
	})
	vm.chainStore = chainstore.New(chainStoreDB)
	return nil
}

func (vm *VM) initLastAccepted(ctx context.Context) (*chain.OutputBlock, error) {
	lastAcceptedHeight, err := vm.chainStore.GetLastAcceptedHeight()
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to load genesis block: %w", err)
	}
	if err == database.ErrNotFound {
		return vm.initGenesisAsLastAccepted(ctx)
	}

	blkBytes, err := vm.chainStore.GetBlockByHeight(lastAcceptedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block at last accepted height: %w", err)
	}

	blk, err := vm.chain.ParseBlock(ctx, blkBytes)
	if err != nil {
		return nil, err
	}

	heightBytes, err := vm.stateDB.Get(chain.HeightKey(vm.metadataManager.HeightPrefix()))
	if err != nil {
		return nil, err
	}
	stateHeight, err := database.ParseUInt64(heightBytes)
	if err != nil {
		return nil, err
	}
	switch {
	case stateHeight == lastAcceptedHeight:
	case stateHeight < lastAcceptedHeight:
		// TODO: we have the blocks, re-process to calculate the last accepted state
		return nil, fmt.Errorf("last accepted state height %d is less than chain height %d", stateHeight, lastAcceptedHeight)
	default:
		// Blocks are written via Pebble with the Sync option (writes through with fsync) and we write blocks
		// before committing state.
		// This should guarantee we never reach this point unless there is a bug or machine crash.
		// TODO: log error and fall back to state sync rather than handling this as a fatal error
		return nil, fmt.Errorf("last accepted state height %d is greater than chain height %d", stateHeight, lastAcceptedHeight)
	}

	return &chain.OutputBlock{
		ExecutionBlock:   blk,
		View:             MerkleDBWithNoopCommit{vm.stateDB},
		ExecutionResults: chain.ExecutionResults{ /* TODO */ },
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

	if err := vm.chainStore.UpdateLastAccepted(genesisExecutionBlk.ID(), 0, genesisExecutionBlk.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to write genesis block: %w", err)
	}

	return &chain.OutputBlock{
		ExecutionBlock:   genesisExecutionBlk,
		View:             MerkleDBWithNoopCommit{vm.stateDB},
		ExecutionResults: chain.ExecutionResults{},
	}, nil
}

type MerkleDBWithNoopCommit struct {
	merkledb.MerkleDB
}

func (m MerkleDBWithNoopCommit) CommitToDB(context.Context) error { return nil }

func (vm *VM) applyOptions(o *Options) error {
	blockSubs := make([]event.Subscription[*chain.ExecutedBlock], len(o.blockSubscriptionFactories))
	for i, factory := range o.blockSubscriptionFactories {
		sub, err := factory.New()
		if err != nil {
			return err
		}
		blockSubs[i] = sub
	}
	vm.snowOptions.WithAcceptedSub(event.SubscriptionFunc[*chain.OutputBlock]{
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
		vm.builder = builder.NewManual(vm.toEngine, vm.snowCtx.Log)
	} else {
		vm.builder = builder.NewTime(vm.toEngine, vm.snowCtx.Log, vm.mempool, func(ctx context.Context, t int64) (int64, int64, error) {
			blk, err := vm.chainIndex.GetPreferredBlock(ctx)
			if err != nil {
				return 0, 0, err
			}
			return blk.Tmstmp, vm.ruleFactory.GetRules(t).GetMinBlockGap(), nil
		})
	}
	vm.snowOptions.WithCloser(func() error {
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
			vm.stop,
		)
		if err != nil {
			return err
		}
		vm.gossiper = txGossiper
		vm.snowOptions.WithVerifiedSub(event.SubscriptionFunc[*chain.OutputBlock]{
			NotifyF: func(_ context.Context, b *chain.OutputBlock) error {
				txGossiper.BlockVerified(b.Timestamp())
				return nil
			},
		})
	}
	vm.snowOptions.WithCloser(func() error {
		vm.gossiper.Done()
		return nil
	})
	return nil
}

func (vm *VM) checkActivity(ctx context.Context) {
	vm.gossiper.Queue(ctx)
	vm.builder.Queue(ctx)
}

func (vm *VM) IsReady() bool {
	select {
	case <-vm.ready:
		return true
	default:
		vm.snowCtx.Log.Info("node is not ready yet")
		return false
	}
}

func (vm *VM) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	if !vm.IsReady() {
		return utils.Repeat[[]byte](nil, len(keys)), utils.Repeat(ErrNotReady, len(keys))
	}
	// Atomic read to ensure consistency
	return vm.stateDB.GetValues(ctx, keys)
}

// markReady
// func (vm *VM) SetState(ctx context.Context, state snow.State) error {
// 	switch state {
// 	case snow.StateSyncing:
// 		vm.Logger().Info("state sync started")
// 		return nil
// 	case snow.Bootstrapping:
// 		// Ensure state sync client marks itself as done if it was never started
// 		syncStarted := vm.StateSyncClient.Started()
// 		if !syncStarted {
// 			// We must check if we finished syncing before starting bootstrapping.
// 			// This should only ever occur if we began a state sync, restarted, and
// 			// were unable to find any acceptable summaries.
// 			syncing, err := vm.GetDiskIsSyncing()
// 			if err != nil {
// 				vm.Logger().Error("could not determine if syncing", zap.Error(err))
// 				return err
// 			}
// 			if syncing {
// 				vm.Logger().Error("cannot start bootstrapping", zap.Error(ErrStateSyncing))
// 				// This is a fatal error that will require retrying sync or deleting the
// 				// node database.
// 				return ErrStateSyncing
// 			}

// 			// If we weren't previously syncing, we force state syncer completion so
// 			// that the node will mark itself as ready.
// 			vm.StateSyncClient.ForceDone()

// 			// TODO: add a config to FATAL here if could not state sync (likely won't be
// 			// able to recover in networks where no one has the full state, bypass
// 			// still starts sync): https://github.com/ava-labs/hypersdk/issues/438
// 		}

// 		// Start the chain syncer and mark the validity window as completed if possible.
// 		seenValidityWindow, err := vm.syncer.Accept(ctx, vm.lastAccepted.ExecutionBlock)
// 		if err != nil {
// 			return err
// 		}
// 		if seenValidityWindow {
// 			vm.seenValidityWindowOnce.Do(func() {
// 				close(vm.seenValidityWindow)
// 			})
// 		}

// 		// Trigger that bootstrapping has started
// 		vm.Logger().Info("bootstrapping started", zap.Bool("state sync started", syncStarted))
// 		return vm.onBootstrapStarted()
// 	case snow.NormalOp:
// 		vm.Logger().
// 			Info("normal operation started", zap.Bool("state sync started", vm.StateSyncClient.Started()))
// 		return vm.onNormalOperationsStarted()
// 	default:
// 		return snow.ErrUnknownState
// 	}
// }

// onBootstrapStarted marks this VM as bootstrapping
func (vm *VM) onBootstrapStarted() error {
	vm.bootstrapped.Set(false)
	return nil
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
	if err := vm.chainStore.UpdateLastAccepted(block.ID(), block.Height(), block.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to write block: %w", err)
	}
	return block, vm.chain.AcceptBlock(ctx, block.ExecutionBlock)
}

func (vm *VM) AcceptDynamicStateSyncBlock(ctx context.Context, block *chain.ExecutionBlock) error {
	return vm.chain.AcceptBlock(ctx, block)
}

func (vm *VM) GetBlock(ctx context.Context, blkID ids.ID) ([]byte, error) {
	return vm.chainStore.GetBlock(blkID)
}

func (vm *VM) GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error) {
	return vm.chainStore.GetBlockIDAtHeight(height)
}

func (vm *VM) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// We should not allow any transactions to be submitted if the VM is not
	// ready yet. We should never reach this point because of other checks but it
	// is good to be defensive.
	if !vm.IsReady() {
		return []error{ErrNotReady}
	}

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
