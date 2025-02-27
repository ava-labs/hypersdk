// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"math"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	internal_fees "github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
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

func GenerateTestExecutedBlocks(
	require *require.Assertions,
	parentID ids.ID,
	chainID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
	numTx int,
) []*chain.ExecutedBlock {
	bh := NewEmptyBalanceHandler(nil)
	rules := genesis.NewDefaultRules()
	feeManager := internal_fees.NewManager(nil)
	executedBlocks := make([]*chain.ExecutedBlock, numBlocks)
	parser := NewTestParser()
	ts := tstate.New(0)
	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{BranchFactor: 2},
	)
	require.NoError(err)
	tstateview := ts.NewView(state.CompletePermissions, db, 0)

	for i := range executedBlocks {
		// generate transactions.
		txs := []*chain.Transaction{}
		base := &chain.Base{
			Timestamp: ((parentTimestamp + timestampOffset*int64(i)/consts.MillisecondsPerSecond) + 1) * consts.MillisecondsPerSecond,
			ChainID:   chainID,
			MaxFee:    math.MaxUint64,
		}
		for range numTx {
			actions := chain.Actions{&TestAction{
				SpecifiedStateKeys: state.Keys{},
				ReadKeys:           [][]byte{},
				WriteKeys:          [][]byte{},
				WriteValues:        [][]byte{},
			}}
			auth := &TestAuth{}
			tx, err := chain.NewTransaction(base, actions, auth)
			require.NoError(err)
			txs = append(txs, tx)
		}
		statelessBlock, err := chain.NewStatelessBlock(
			parentID,
			parentTimestamp+timestampOffset*int64(i),
			parentHeight+1+uint64(i),
			txs,
			ids.Empty,
			nil,
		)
		require.NoError(err)
		parentID = statelessBlock.GetID()

		results := []*chain.Result{}
		for _, tx := range txs {
			res, err := tx.Execute(context.Background(), feeManager, bh, rules, tstateview, parentTimestamp+timestampOffset*int64(i))
			require.NoError(err)
			results = append(results, res)
		}
		// marshal and unmarshal the block in order to remove all non-persisting data members ( i.e. statekeys )
		blkBytes, err := statelessBlock.Marshal()
		require.NoError(err)
		statelessBlock, err = chain.UnmarshalBlock(blkBytes, parser)
		require.NoError(err)

		blk := chain.NewExecutedBlock(
			statelessBlock,
			results,
			fees.Dimensions{},
			fees.Dimensions{},
		)
		executedBlocks[i] = blk
	}
	return executedBlocks
}
