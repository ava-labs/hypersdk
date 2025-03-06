// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"encoding/binary"
	"fmt"
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
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

func GenerateEmptyExecutedBlocks(
	require *require.Assertions,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
) []*chain.ExecutedBlock {
	executedBlocks := make([]*chain.ExecutedBlock, numBlocks)
	for i := range executedBlocks {
		statelessBlock, err := chain.NewStatelessBlock(
			parentID,
			parentTimestamp+timestampOffset*int64(i),
			parentHeight+1+uint64(i),
			[]*chain.Transaction{},
			ids.Empty,
			nil,
		)
		require.NoError(err)
		parentID = statelessBlock.GetID()

		blk := chain.NewExecutedBlock(
			statelessBlock,
			[]*chain.Result{},
			fees.Dimensions{},
			fees.Dimensions{},
		)
		executedBlocks[i] = blk
	}
	return executedBlocks
}

// TXGenerator is a function that generates a valid TX that contains
// [actions]. [factory] will be the signer of the TX.
type TxGenerator func(actions []chain.Action, factory chain.AuthFactory) (*chain.Transaction, error)

// BlockBenchmarkHelper is a VM-agnostic helper that allows the tester to
// generate the genesis commit and the list of transactions for each block.
type BlockBenchmarkHelper interface {
	// GenerateGenesis returns the genesis that will be used to create the
	// genesis commit of the chain. This method will be called exactly once at
	// the beginning of the benchmark and will be called before GenerateTxList.
	//
	// Any factories that will be needed for GenerateTxList should be created
	// here and part of the genesis (i.e. the allocation list).
	GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error)
	// GenerateTxList should return a list of transactions of length [numOfTxsPerBlock].
	GenerateTxList(numOfTxsPerBlock uint64, txGenerator TxGenerator) ([]*chain.Transaction, error)
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
	blockBenchmarkHelper BlockBenchmarkHelper,
	parentView state.View,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
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
		timestamp := parentTimestamp + timestampOffset*(int64(i)+1)
		height := parentHeight + 1 + uint64(i)

		feeBytes, err := parentView.GetValue(ctx, chain.FeeKey(metadataManager.FeePrefix()))
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

		txs, err := blockBenchmarkHelper.GenerateTxList(numOfTxsPerBlock, txGenerator)
		if err != nil {
			return nil, err
		}

		ts := tstate.New(0)
		for _, tx := range txs {
			stateKeys, err := tx.StateKeys(balanceHandler)
			if err != nil {
				return nil, err
			}
			tsv := ts.NewView(stateKeys, parentView, 0)
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

		tsv := ts.NewView(state.CompletePermissions, parentView, 0)
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

		root, err := parentView.GetMerkleRoot(ctx)
		if err != nil {
			return nil, err
		}

		parentView, err = parentView.NewView(ctx, merkledb.ViewChanges{
			MapOps:       ts.ChangedKeys(),
			ConsumeBytes: true,
		})
		if err != nil {
			return nil, err
		}

		statelessBlock, err := chain.NewStatelessBlock(
			parentID,
			timestamp,
			height,
			txs,
			root,
			nil,
		)
		if err != nil {
			return nil, err
		}
		parentID = statelessBlock.GetID()

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
	AuthVM          chain.AuthVM

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
		test.AuthVM,
		test.MetadataManager,
		test.BalanceHandler,
		&validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
		metrics,
		test.Config,
	)

	genesis, err := test.BlockBenchmarkHelper.GenerateGenesis(test.NumOfTxsPerBlock)
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

	blocks, err := GenerateExecutionBlocks(
		ctx,
		test.RuleFactory.GetRules(time.Now().UnixMilli()),
		test.MetadataManager,
		test.BalanceHandler,
		test.BlockBenchmarkHelper,
		db,
		genesisExecutionBlk.GetID(),
		genesisExecutionBlk.GetHeight(),
		genesisExecutionBlk.GetTimestamp(),
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
