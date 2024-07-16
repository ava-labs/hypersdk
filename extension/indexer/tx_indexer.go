// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

var _ AcceptedSubscriber = (*TxIndexer)(nil)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)
)

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

func (i *TxIndexer) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	batch := i.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
		if i.enabled {
			if err := i.storeTransaction(
				batch,
				tx.ID(),
				timestamp,
				result.Success,
				result.Units,
				result.Fee,
			); err != nil {
				return err
			}
		}

		if result.Success {
			if err := i.successfulTxIndexer.Accepted(ctx, tx, result); err != nil {
				return err
			}
		}
	}

	return batch.Write()
}

func (*TxIndexer) storeTransaction(
	batch database.KeyValueWriter,
	txID ids.ID,
	t int64,
	success bool,
	units fees.Dimensions,
	fee uint64,
) error {
	v := make([]byte, consts.Uint64Len+1+fees.DimensionsLen+consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(t))
	if success {
		v[consts.Uint64Len] = successByte
	} else {
		v[consts.Uint64Len] = failureByte
	}
	copy(v[consts.Uint64Len+1:], units.Bytes())
	binary.BigEndian.PutUint64(v[consts.Uint64Len+1+fees.DimensionsLen:], fee)
	return batch.Put(txID[:], v)
}

func (i *TxIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	v, err := i.db.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, fees.Dimensions{}, 0, nil
	}
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	t := int64(binary.BigEndian.Uint64(v))
	success := true
	if v[consts.Uint64Len] == failureByte {
		success = false
	}
	d, err := fees.UnpackDimensions(v[consts.Uint64Len+1 : consts.Uint64Len+1+fees.DimensionsLen])
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	fee := binary.BigEndian.Uint64(v[consts.Uint64Len+1+fees.DimensionsLen:])
	return true, t, success, d, fee, nil
}
