// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

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
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

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

// TxGenerator is a function that generates a valid TX that contains
// [actions]. [factory] will be the signer of the TX.
type TxGenerator func(actions []chain.Action, factory chain.AuthFactory) (*chain.Transaction, error)

// TxListGenerator is a function that should return a list of valid TXs of
// length [numOfTxsPerBlock].
type TxListGenerator func(numOfTxsPerBlock uint64, txGenerator TxGenerator) ([]*chain.Transaction, error)

func EmptyTxListGenerator(uint64, TxGenerator) ([]*chain.Transaction, error) {
	return []*chain.Transaction{}, nil
}

// BlockBenchmarkHelper initializes a BlockBenchmark test by returning the
// genesis and TxListGenerator to be used.
type BlockBenchmarkHelper func(numOfTxsPerBlock uint64) (genesis.Genesis, TxListGenerator, error)

func NoopBlockBenchmarkHelper(uint64) (genesis.Genesis, TxListGenerator, error) {
	return genesis.NewDefaultGenesis([]*genesis.CustomAllocation{}), EmptyTxListGenerator, nil
}

// parentContext holds values relating to the parent block that is required
// for producing the child block.
type parentContext struct {
	view      state.View
	id        ids.ID
	height    uint64
	timestamp int64
}

// GenerateExecutionBlocks generates [numBlocks] execution blocks with
// [txsPerBlock] transactions per block.
//
// Block production is a simplified version of Builder; we execute
// transactions followed by writing the chain metadata to the state diff.
func GenerateExecutionBlocks(
	ctx context.Context,
	rules chain.Rules,
	metadataManager chain.MetadataManager,
	balanceHandler chain.BalanceHandler,
	txListGenerator TxListGenerator,
	parentCtx *parentContext,
	numBlocks uint64,
	numOfTxsPerBlock uint64,
) ([]*chain.ExecutionBlock, error) {
	var timestampOffset int64
	timestampOffset = rules.GetMinEmptyBlockGap()
	if numOfTxsPerBlock > 0 {
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

		txGenerator := func(actions []chain.Action, factory chain.AuthFactory) (*chain.Transaction, error) {
			units, err := chain.EstimateUnits(rules, actions, factory)
			if err != nil {
				return nil, err
			}

			maxFee, err := fees.MulSum(unitPrices, units)
			if err != nil {
				return nil, err
			}

			txData := chain.NewTxData(
				chain.Base{
					Timestamp: utils.UnixRMilli(timestamp, rules.GetValidityWindow()),
					ChainID:   rules.GetChainID(),
					MaxFee:    maxFee,
				},
				actions,
			)

			return txData.Sign(factory)
		}

		txs, err := txListGenerator(numOfTxsPerBlock, txGenerator)
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

		root, err := parentCtx.view.GetMerkleRoot(ctx)
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
			root,
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

// BlockBenchmark is a parameterized benchmark. It generates [NumOfBlocks] blocks
// with [NumOfTxsPerBlock] transactions per block, and then calls
// Processor.Execute to process the block list [b.N] times.
type BlockBenchmark struct {
	MetadataManager chain.MetadataManager
	BalanceHandler  chain.BalanceHandler
	RuleFactory     chain.RuleFactory
	AuthEngines     chain.AuthEngines

	BlockBenchmarkHelper BlockBenchmarkHelper

	Config                chain.Config
	AuthVerificationCores int

	// [NumOfBlocks] is set as a hyperparameter to avoid using [b.N] to generate
	// the number of blocks to run the benchmark on.
	NumOfBlocks      uint64
	NumOfTxsPerBlock uint64
}

func (test *BlockBenchmark) Run(ctx context.Context, b *testing.B) {
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

	numOfAuthVerificationJobs := 100

	processorWorkers := workers.NewSerial()
	if test.AuthVerificationCores > 1 {
		processorWorkers = workers.NewParallel(test.AuthVerificationCores, numOfAuthVerificationJobs)
	}

	processor := chain.NewProcessor(
		trace.Noop,
		&logging.NoLog{},
		test.RuleFactory,
		processorWorkers,
		test.AuthEngines,
		test.MetadataManager,
		test.BalanceHandler,
		&validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
		metrics,
		test.Config,
	)

	genesis, txListGenerator, err := test.BlockBenchmarkHelper(test.NumOfTxsPerBlock)
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
		test.RuleFactory.GetRules(time.Now().UnixMilli()),
		test.MetadataManager,
		test.BalanceHandler,
		txListGenerator,
		parentCtx,
		test.NumOfBlocks,
		test.NumOfTxsPerBlock,
	)
	r.NoError(err)

	var parentView merkledb.View
	parentView = db
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < int(test.NumOfBlocks); j++ {
			outputBlock, err := processor.Execute(
				ctx,
				parentView,
				blocks[j],
				true,
			)
			r.NoError(err)
			// we update [parentView] so that the view produced by [outputBlock]
			// is the parent view of the next block
			parentView = outputBlock.View
		}
		// we reset [parentView] to the genesis view
		parentView = db
	}
}
