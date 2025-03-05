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

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var _ TxListGenerator = (*NoopTxListGenerator)(nil)

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

// TxListGenerator is used to populate blocks for benchmarking.
type TxListGenerator interface {
	// Generate returns a list of transactions that will be the TX list of a
	// block (can be empty or can have a length not equal to [numOfTxs]).
	//
	// [factories] is a list of unique accounts whose length is equal to [numOfTxs].
	Generate(
		rules chain.Rules,
		timestamp int64,
		unitPrices fees.Dimensions,
		factories []chain.AuthFactory,
		numOfTxs uint64,
	) ([]*chain.Transaction, error)
}

// NoopTxListGenerator is a TxListGenerator that generates an empty TX list
type NoopTxListGenerator struct{}

func (NoopTxListGenerator) Generate(
	chain.Rules,
	int64,
	fees.Dimensions,
	[]chain.AuthFactory,
	uint64,
) ([]*chain.Transaction, error) {
	return []*chain.Transaction{}, nil
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
	factories []chain.AuthFactory,
	parentView state.View,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	numBlocks uint64,
	numOfTxsPerBlock uint64,
) ([]*chain.ExecutionBlock, error) {
	var timestampOffset int64
	switch numOfTxsPerBlock {
	case 0:
		timestampOffset = rules.GetMinEmptyBlockGap()
	default:
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

		txs, err := txListGenerator.Generate(rules, timestamp, unitPrices, factories, numOfTxsPerBlock)
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
	TxListGenerator TxListGenerator

	Config                chain.Config
	AuthVerificationCores int

	// [NumOfBlocks] is set as a hyperparameter to avoid using [b.N] to generate
	// the number of blocks to run the benchmark on.
	NumOfBlocks uint64
	// [NumOfTxsPerBlock] is also equal to the number of unique factories the
	// benchmark will create.
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

	var (
		processorWorkers          workers.Workers
		numOfAuthVerificationJobs = 100
		allocBalance              = 1_000_000_000
	)

	switch test.AuthVerificationCores {
	case 0, 1:
		processorWorkers = workers.NewSerial()
	default:
		processorWorkers = workers.NewParallel(
			test.AuthVerificationCores,
			numOfAuthVerificationJobs,
		)
	}

	factories := []chain.AuthFactory{}
	customAllocs := []*genesis.CustomAllocation{}
	for range test.NumOfTxsPerBlock {
		pk, err := ed25519.GeneratePrivateKey()
		r.NoError(err)

		factory := auth.NewED25519Factory(pk)
		factories = append(factories, factory)

		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: factory.Address(),
			Balance: uint64(allocBalance),
		})
	}
	genesis := genesis.NewDefaultGenesis(customAllocs)

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

	genesisExecutionBlk, genesisView, err := chain.NewGenesisStateDiff(
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

	// Generate blocks
	blocks, err := GenerateExecutionBlocks(
		ctx,
		test.RuleFactory.GetRules(time.Now().UnixMilli()),
		test.MetadataManager,
		test.BalanceHandler,
		test.TxListGenerator,
		factories,
		db,
		genesisExecutionBlk.GetID(),
		genesisExecutionBlk.GetHeight(),
		genesisExecutionBlk.GetTimestamp(),
		test.NumOfBlocks,
		test.NumOfTxsPerBlock,
	)
	r.NoError(err)

	var view merkledb.View
	view = db
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < int(test.NumOfBlocks); j++ {
			outputBlock, err := processor.Execute(
				ctx,
				view,
				blocks[j],
				false,
			)
			r.NoError(err)
			view = outputBlock.View
		}
		view = db
	}
}
