/*
 * Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

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

var (
	_ AcceptedSubscriber = (*TxDBIndexer)(nil)
	_ AcceptedSubscriber = (*NoOpTxIndexer)(nil)

	failureByte = byte(0x0)
	successByte = byte(0x1)
)

type TxIndexer interface {
	AcceptedSubscriber
	GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error)
}

type TxDBIndexer struct {
	db database.Database
}

func NewTxDBIndexer(db database.Database) *TxDBIndexer {
	return &TxDBIndexer{db: db}
}

func (t *TxDBIndexer) Accepted(_ context.Context, blk *chain.StatelessBlock) error {
	batch := t.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
		if err := t.storeTransaction(
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

	return batch.Write()
}

func (*TxDBIndexer) storeTransaction(
	batch database.KeyValueWriter,
	txID ids.ID,
	timestamp int64,
	success bool,
	units fees.Dimensions,
	fee uint64,
) error {
	v := make([]byte, consts.Uint64Len+1+fees.DimensionsLen+consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(timestamp))
	if success {
		v[consts.Uint64Len] = successByte
	} else {
		v[consts.Uint64Len] = failureByte
	}
	copy(v[consts.Uint64Len+1:], units.Bytes())
	binary.BigEndian.PutUint64(v[consts.Uint64Len+1+fees.DimensionsLen:], fee)
	return batch.Put(txID[:], v)
}

func (t *TxDBIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	v, err := t.db.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, fees.Dimensions{}, 0, nil
	}
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	timestamp := int64(binary.BigEndian.Uint64(v))
	success := true
	if v[consts.Uint64Len] == failureByte {
		success = false
	}
	d, err := fees.UnpackDimensions(v[consts.Uint64Len+1 : consts.Uint64Len+1+fees.DimensionsLen])
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	fee := binary.BigEndian.Uint64(v[consts.Uint64Len+1+fees.DimensionsLen:])
	return true, timestamp, success, d, fee, nil
}

type NoOpTxIndexer struct{}

func NewNoOpTxIndexer() *NoOpTxIndexer {
	return &NoOpTxIndexer{}
}

func (*NoOpTxIndexer) Accepted(_ context.Context, _ *chain.StatelessBlock) error {
	return nil
}

func (*NoOpTxIndexer) GetTransaction(_ ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return false, 0, false, fees.Dimensions{}, 0, errors.New("tx indexer not enabled")
}
