// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

func GenerateEmptyExecutedBlocks(
	require *require.Assertions,
	parentID ids.ID,
	chainID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
	numTxsPerBlock int,
) []*chain.ExecutedBlock {
	bh := balance.NewPrefixBalanceHandler([]byte{0})
	rules := genesis.NewDefaultRules()
	feeManager := internalfees.NewManager(nil)
	executedBlocks := make([]*chain.ExecutedBlock, numBlocks)
	parser := NewTestParser()
	ts := tstate.New(0)
	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{BranchFactor: merkledb.BranchFactor16},
	)
	require.NoError(err)

	tstateview := ts.NewView(state.CompletePermissions, db, 0)
	require.NoError(tstateview.Insert(context.Background(), bh.BalanceKey(NewDummyTestAuth().Sponsor()), binary.BigEndian.AppendUint64(nil, math.MaxUint64)))

	actions := NewDummyTestActions(numBlocks * numTxsPerBlock)
	for blockIndex := range executedBlocks {
		blkTimestamp := parentTimestamp + timestampOffset*int64(blockIndex)
		// set transaction timestamp to the next whole second multiplier past the block timestamp.
		txTimestamp := (parentTimestamp/consts.MillisecondsPerSecond + 1) * consts.MillisecondsPerSecond
		// generate transactions.
		txs := make([]*chain.Transaction, 0, numTxsPerBlock)
		base := chain.Base{
			Timestamp: txTimestamp,
			ChainID:   chainID,
			MaxFee:    math.MaxUint64,
		}
		for txIndex := range numTxsPerBlock {
			actions := []chain.Action{actions[blockIndex*numTxsPerBlock+txIndex]}
			auth := NewDummyTestAuth()
			tx, err := chain.NewTransaction(base, actions, auth)
			require.NoError(err)
			txs = append(txs, tx)
		}
		statelessBlock, err := chain.NewStatelessBlock(
			parentID,
			blkTimestamp,
			parentHeight+1+uint64(blockIndex),
			txs,
			ids.Empty,
			nil,
		)
		require.NoError(err)
		parentID = statelessBlock.GetID()

		results := []*chain.Result{}
		for _, tx := range txs {
			res, err := tx.Execute(context.Background(), feeManager, bh, rules, tstateview, blkTimestamp)
			require.NoError(err)
			results = append(results, res)
		}
		// marshal and unmarshal the block in order to remove all non-persisting data members ( i.e. statekeys )
		blkBytes := statelessBlock.GetBytes()
		require.NoError(err)
		statelessBlock, err = chain.UnmarshalBlock(blkBytes, parser)
		require.NoError(err)

		blk := chain.NewExecutedBlock(
			statelessBlock,
			results,
			fees.Dimensions{},
			fees.Dimensions{},
		)
		executedBlocks[blockIndex] = blk
	}
	return executedBlocks
}

// parentContext holds values relating to the parent block that is required
// for producing the child block.
type parentContext struct {
	view      state.View
	id        ids.ID
	height    uint64
	timestamp int64
}

// GenerateExecutionBlocks generates numBlocks execution blocks with
// txsPerBlock transactions per block.
//
// Block production is a simplified version of Builder. We execute
// transactions followed by writing the chain metadata to the state diff.
func GenerateExecutionBlocks[T any](
	ctx context.Context,
	ruleFactory chain.RuleFactory,
	metadataManager chain.MetadataManager,
	balanceHandler chain.BalanceHandler,
	factories []chain.AuthFactory,
	stateAccessDistributor StateAccessDistributor[T],
	keys []T,
	parentCtx *parentContext,
	numBlocks uint64,
	numTxsPerBlock uint64,
) ([]*chain.ExecutionBlock, error) {
	var timestampOffset int64
	rules := ruleFactory.GetRules(parentCtx.timestamp)
	timestampOffset = rules.GetMinEmptyBlockGap()
	if numTxsPerBlock > 0 {
		timestampOffset = rules.GetMinBlockGap()
	}
	executionBlocks := make([]*chain.ExecutionBlock, numBlocks)
	for i := range executionBlocks {
		timestamp := parentCtx.timestamp + timestampOffset*(int64(i)+1)
		height := parentCtx.height + 1 + uint64(i)

		feeBytes, err := parentCtx.view.GetValue(ctx, chain.FeeKey(metadataManager.FeePrefix()))
		if err != nil {
			return nil, err
		}
		feeManager := internalfees.NewManager(feeBytes)
		feeManager = feeManager.ComputeNext(timestamp, rules)
		unitPrices := feeManager.UnitPrices()

		txs, err := stateAccessDistributor.Generate(
			numTxsPerBlock,
			factories,
			keys,
			unitPrices,
			timestamp,
		)
		if err != nil {
			return nil, err
		}

		ts := tstate.New(0)
		for _, tx := range txs {
			stateKeys, err := tx.StateKeys(balanceHandler)
			if err != nil {
				return nil, err
			}
			tsv := ts.NewView(stateKeys, parentCtx.view, 0)
			if err := tx.PreExecute(ctx, feeManager, balanceHandler, rules, tsv, timestamp); err != nil {
				return nil, err
			}
			result, err := tx.Execute(ctx, feeManager, balanceHandler, rules, tsv, timestamp)
			if err != nil {
				return nil, err
			}
			ok, d := feeManager.Consume(result.Units, rules.GetMaxBlockUnits())
			if !ok {
				return nil, fmt.Errorf("consumed too many units in dim %d", d)
			}
			tsv.Commit()
		}

		tsv := ts.NewView(state.CompletePermissions, parentCtx.view, 0)
		if err := tsv.Insert(ctx, chain.HeightKey(metadataManager.HeightPrefix()), binary.BigEndian.AppendUint64(nil, height)); err != nil {
			return nil, err
		}
		if err := tsv.Insert(ctx, chain.TimestampKey(metadataManager.TimestampPrefix()), binary.BigEndian.AppendUint64(nil, uint64(timestamp))); err != nil {
			return nil, err
		}
		if err := tsv.Insert(ctx, chain.FeeKey(metadataManager.FeePrefix()), feeManager.Bytes()); err != nil {
			return nil, err
		}
		tsv.Commit()

		parentRoot, err := parentCtx.view.GetMerkleRoot(ctx)
		if err != nil {
			return nil, err
		}

		parentCtx.view, err = parentCtx.view.NewView(ctx, merkledb.ViewChanges{
			MapOps:       ts.ChangedKeys(),
			ConsumeBytes: true,
		})
		if err != nil {
			return nil, err
		}

		statelessBlock, err := chain.NewStatelessBlock(
			parentCtx.id,
			timestamp,
			height,
			txs,
			parentRoot,
			nil,
		)
		if err != nil {
			return nil, err
		}
		parentCtx.id = statelessBlock.GetID()

		blk := chain.NewExecutionBlock(statelessBlock)
		executionBlocks[i] = blk
	}
	return executionBlocks, nil
}

// BlockBenchmark is a parameterized benchmark. It generates NumBlocks
// with NumTxsPerBlock, and then calls Processor.Execute to process the block
// list b.N times.
type BlockBenchmark[T any] struct {
	MetadataManager chain.MetadataManager
	BalanceHandler  chain.BalanceHandler
	RuleFactory     chain.RuleFactory
	AuthEngines     chain.AuthEngines

	Config                chain.Config
	AuthVerificationCores int

	GenesisF               GenesisGenerator[T]
	StateAccessDistributor StateAccessDistributor[T]

	// NumBlocks is set as a hyperparameter to avoid using b.N to generate
	// the number of blocks to run the benchmark on.
	NumBlocks      uint64
	NumTxsPerBlock uint64
}

func (test *BlockBenchmark[T]) Run(ctx context.Context, b *testing.B) {
	r := require.New(b)

	metrics, err := chain.NewMetrics(prometheus.NewRegistry())
	r.NoError(err)

	db, err := merkledb.New(
		ctx,
		memdb.New(),
		merkledb.Config{
			BranchFactor: merkledb.BranchFactor16,
			Tracer:       trace.Noop,
		},
	)
	r.NoError(err)

	numAuthVerificationJobs := 100

	processorWorkers := workers.NewSerial()
	if test.AuthVerificationCores > 1 {
		processorWorkers = workers.NewParallel(test.AuthVerificationCores, numAuthVerificationJobs)
	}

	chainIndex := &validitywindowtest.MockChainIndex[*chain.Transaction]{}
	validityWindow := validitywindow.NewTimeValidityWindow(
		ctx,
		logging.NoLog{},
		trace.Noop,
		chainIndex,
		nil,
		func(timestamp int64) int64 {
			return test.RuleFactory.GetRules(timestamp).GetValidityWindow()
		},
	)

	processor := chain.NewProcessor(
		trace.Noop,
		&logging.NoLog{},
		test.RuleFactory,
		processorWorkers,
		test.AuthEngines,
		test.MetadataManager,
		test.BalanceHandler,
		validityWindow,
		metrics,
		test.Config,
	)

	factories, keys, genesis, err := test.GenesisF(test.NumTxsPerBlock)
	r.NoError(err)

	genesisExecutionBlk, genesisView, err := chain.NewGenesisCommit(
		ctx,
		db,
		genesis,
		test.MetadataManager,
		test.BalanceHandler,
		test.RuleFactory,
		trace.Noop,
		logging.NoLog{},
	)
	r.NoError(err)
	r.NoError(genesisView.CommitToDB(ctx))

	parentCtx := &parentContext{
		view:      db,
		id:        genesisExecutionBlk.GetID(),
		height:    genesisExecutionBlk.GetHeight(),
		timestamp: genesisExecutionBlk.GetTimestamp(),
	}

	blocks, err := GenerateExecutionBlocks(
		ctx,
		test.RuleFactory,
		test.MetadataManager,
		test.BalanceHandler,
		factories,
		test.StateAccessDistributor,
		keys,
		parentCtx,
		test.NumBlocks,
		test.NumTxsPerBlock,
	)
	r.NoError(err)

	// Store all produced blocks to chainIndex
	chainIndex.Set(genesisExecutionBlk.GetID(), genesisExecutionBlk)
	for _, blk := range blocks {
		chainIndex.Set(blk.GetID(), blk)
	}

	var parentView merkledb.View
	parentView = db
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < int(test.NumBlocks); j++ {
			outputBlock, err := processor.Execute(
				ctx,
				parentView,
				blocks[j],
				true,
			)
			r.NoError(err)
			// we update parentView so that the view produced by outputBlock
			// is the parent view of the next block
			parentView = outputBlock.View
		}
		// we reset parentView to the genesis view
		parentView = db
	}

	numBlocksExecuted := test.NumBlocks * uint64(b.N)
	numTxsExecuted := numBlocksExecuted * test.NumTxsPerBlock
	b.ReportMetric(float64(numTxsExecuted)/b.Elapsed().Seconds(), "tps")
	b.ReportMetric(float64(numBlocksExecuted)/b.Elapsed().Seconds(), "blocks/s")
}

// GenesisGenerator is a helper function that returns the following:
//
// 1. numTxsPerBlock auth factories
// 2. the set of keys used for state access distribution
// 3. the genesis of the chain
//
// The state access set is returned here since its possible that the state
// access set is derived from the generated auth factories
type GenesisGenerator[T any] func(numTxsPerBlock uint64) ([]chain.AuthFactory, []T, genesis.Genesis, error)

// ActionConstructor converts a state access key into an action.
type ActionConstructor[T any] interface {
	// Generate takes in a key from the state access set and returns an action.
	// The action returned should be unique relative to the state access key.
	Generate(k T, nonce uint64) chain.Action
}

// StateAccessDistributor generates a list of transactions whose state accesses
// vary depending on the implementation.
type StateAccessDistributor[T any] interface {
	// Generate returns a list of transactions
	//
	// For each transaction, Generate should select a key from the state access set
	// (keys) and construct an action. This action is then used to create a
	// transaction.
	//
	// Implementations of Generate will vary in the way they select state access
	// keys (e.g. using unique keys, using the same key, etc.).
	Generate(
		numTxs uint64,
		factories []chain.AuthFactory,
		keys []T,
		unitPrices fees.Dimensions,
		timestamp int64,
	) ([]*chain.Transaction, error)
}

type NoopDistributor[T any] struct{}

func (NoopDistributor[T]) Generate(uint64, []chain.AuthFactory, []T, fees.Dimensions, int64) ([]*chain.Transaction, error) {
	return nil, nil
}

// ParallelDistribution generates a list of transactions that are each assigned
// a unique state access key. When using pessimistic concurrency control, this
// allows for the transactions to be executed in parallel.
type ParallelDistributor[T any] struct {
	nonce             uint64
	ruleFactory       chain.RuleFactory
	actionConstructor ActionConstructor[T]
}

func NewParallelDistributor[T any](actionConstructor ActionConstructor[T], ruleFactory chain.RuleFactory) *ParallelDistributor[T] {
	return &ParallelDistributor[T]{
		actionConstructor: actionConstructor,
		ruleFactory:       ruleFactory,
	}
}

func (p *ParallelDistributor[T]) Generate(
	numTxs uint64,
	factories []chain.AuthFactory,
	keys []T,
	unitPrices fees.Dimensions,
	timestamp int64,
) ([]*chain.Transaction, error) {
	if int(numTxs) != len(keys) || int(numTxs) != len(factories) {
		return nil, ErrMismatchedKeysAndFactoriesLen
	}

	txs := make([]*chain.Transaction, numTxs)
	for i := range numTxs {
		action := p.actionConstructor.Generate(keys[i], p.nonce)
		p.nonce++

		tx, err := chain.GenerateTransaction(
			p.ruleFactory,
			unitPrices,
			timestamp,
			[]chain.Action{action},
			factories[i],
		)
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

// SerialDistribution generates a list of transactions that are each assigned
// the same state access key. As a result, the transactions must be executed serially.
type SerialDistributor[T any] struct {
	nonce             uint64
	ruleFactory       chain.RuleFactory
	actionConstructor ActionConstructor[T]
}

func NewSerialDistributor[T any](actionConstructor ActionConstructor[T], ruleFactory chain.RuleFactory) *SerialDistributor[T] {
	return &SerialDistributor[T]{
		actionConstructor: actionConstructor,
		ruleFactory:       ruleFactory,
	}
}

func (s *SerialDistributor[T]) Generate(
	numTxs uint64,
	factories []chain.AuthFactory,
	keys []T,
	unitPrices fees.Dimensions,
	timestamp int64,
) ([]*chain.Transaction, error) {
	if len(keys) != 1 {
		return nil, ErrSingleKeyLengthOnly
	}

	txs := make([]*chain.Transaction, numTxs)
	for i := range numTxs {
		action := s.actionConstructor.Generate(keys[0], s.nonce)
		s.nonce++

		tx, err := chain.GenerateTransaction(
			s.ruleFactory,
			unitPrices,
			timestamp,
			[]chain.Action{action},
			factories[i],
		)
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

// ZipfDistribution generates a list of transactions whose state access key is
// sampled from a zipf distribution.
type ZipfDistributor[T any] struct {
	nonce             uint64
	ruleFactory       chain.RuleFactory
	actionConstructor ActionConstructor[T]
}

func NewZipfDistributor[T any](actionConstructor ActionConstructor[T], ruleFactory chain.RuleFactory) *ZipfDistributor[T] {
	return &ZipfDistributor[T]{
		actionConstructor: actionConstructor,
		ruleFactory:       ruleFactory,
	}
}

func (z *ZipfDistributor[T]) Generate(
	numTxs uint64,
	factories []chain.AuthFactory,
	keys []T,
	unitPrices fees.Dimensions,
	timestamp int64,
) ([]*chain.Transaction, error) {
	if int(numTxs) != len(keys) || int(numTxs) != len(factories) {
		return nil, ErrMismatchedKeysAndFactoriesLen
	}

	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	sZipf := 1.01
	vZipf := 2.7
	zipfGen := rand.NewZipf(zipfSeed, sZipf, vZipf, numTxs-1)

	txs := make([]*chain.Transaction, numTxs)
	for i := range numTxs {
		action := z.actionConstructor.Generate(keys[zipfGen.Uint64()], z.nonce)
		z.nonce++

		tx, err := chain.GenerateTransaction(
			z.ruleFactory,
			unitPrices,
			timestamp,
			[]chain.Action{action},
			factories[i],
		)
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}
