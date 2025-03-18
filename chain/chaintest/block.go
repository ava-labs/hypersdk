// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"encoding/binary"
	"math"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

// GenerateTestExecutedBlocks creates [numBlocks] executed block for testing purposes, each with [numTx] transactions.
// these blocks are not semantically valid for execution, and are meant execlusively for serialization testing purposes.
func GenerateTestExecutedBlocks(
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
