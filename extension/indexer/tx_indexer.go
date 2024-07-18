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
	_ AcceptedSubscriber = (*txIndexer)(nil)
	_ AcceptedSubscriber = (*noopTxIndexer)(nil)
)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)
)

type TxIndexer interface {
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
	GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error)
}

type txIndexer struct {
	db database.Database
}

func NewTxIndexer(db database.Database) *txIndexer {
	return &txIndexer{db: db}
}

func (i *txIndexer) Accepted(_ context.Context, blk *chain.StatelessBlock) error {
	batch := i.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
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

	return batch.Write()
}

func (*txIndexer) storeTransaction(
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

func (i *txIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
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

type noopTxIndexer struct{}

func NewNoOpTxIndexer() *noopTxIndexer {
	return &noopTxIndexer{}
}

func (*noopTxIndexer) Accepted(_ context.Context, _ *chain.StatelessBlock) error {
	return nil
}

func (*noopTxIndexer) GetTransaction(_ ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return false, 0, false, fees.Dimensions{}, 0, errors.New("tx indexer not enabled")
}
