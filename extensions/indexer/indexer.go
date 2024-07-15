// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

type AcceptedIndexer interface {
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
}

type SuccessfulTxIndexer interface {
	Accepted(ctx context.Context, tx *chain.Transaction, result *chain.Result) error
}

type TxIndexer struct {
	db                  database.Database
	enabled             bool
	successfulTxIndexer SuccessfulTxIndexer
}

func NewTxIndexer(
	db database.Database,
	enabled bool,
	successfulTxIndexer SuccessfulTxIndexer,
) *TxIndexer {
	return &TxIndexer{
		db:                  db,
		enabled:             enabled,
		successfulTxIndexer: successfulTxIndexer,
	}
}

func (indexer *TxIndexer) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	batch := indexer.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for i, tx := range blk.Txs {
		result := results[i]
		if indexer.enabled {
			err := StoreTransaction(
				ctx,
				batch,
				tx.ID(),
				timestamp,
				result.Success,
				result.Units,
				result.Fee,
			)
			if err != nil {
				return err
			}
		}

		if result.Success {
			if err := indexer.successfulTxIndexer.Accepted(ctx, tx, result); err != nil {
				return err
			}
		}
	}

	return batch.Write()
}

func (indexer *TxIndexer) GetTransaction(
	ctx context.Context,
	txID ids.ID,
) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return GetTransaction(ctx, indexer.db, txID)
}
