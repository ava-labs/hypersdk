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
	_ AcceptedSubscriber = (*StandardDBIndexer)(nil)
	_ StandardIndexer    = (*StandardDBIndexer)(nil)
)

type StandardIndexer interface {
	TxIndexer
	AcceptedStateful(context.Context, *chain.StatefulBlock) error
	GetBlockByHeight(height uint64) ([]byte, error)
	GetBlock(blockID ids.ID) ([]byte, error)
	BlockAlreadyIndexed(height uint64) bool
}

type StandardDBIndexer struct {
	db database.Database
}

func NewStandardDBIndexer(db database.Database) *StandardDBIndexer {
	return &StandardDBIndexer{db: db}
}

func (s *StandardDBIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	v, err := s.db.Get(txID[:])
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

func (s *StandardDBIndexer) Accepted(_ context.Context, blk *chain.StatelessBlock) error {
	batch := s.db.NewBatch()
	defer batch.Reset()

	// Store block bytes
	if err := s.storeBlock(batch, blk); err != nil {
		return err
	}
	// Map block ID to block height
	if err := s.mapBlockIDToHeight(batch, blk.ID(), blk.Hght); err != nil {
		return err
	}
	// Store TX
	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
		if err := s.storeTransaction(
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

func (s *StandardDBIndexer) AcceptedStateful(_ context.Context, blk *chain.StatefulBlock) error {
	batch := s.db.NewBatch()
	defer batch.Reset()

	// Store block itself
	if err := s.storeStatefulBlock(batch, blk); err != nil {
		return err
	}
	blkID, err := blk.ID()
	if err != nil {
		return err
	}
	// Map block ID to block height
	if err = s.mapBlockIDToHeight(batch, blkID, blk.Hght); err != nil {
		return err
	}
	// Store TX
	timestamp := blk.Tmstmp
	for _, tx := range blk.Txs {
		if err := s.storeTransactionStateful(batch, tx.ID(), timestamp); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (s *StandardDBIndexer) GetBlockByHeight(height uint64) ([]byte, error) {
	k := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(k, height)
	v, err := s.db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (s *StandardDBIndexer) GetBlock(blockID ids.ID) ([]byte, error) {
	// Get height
	v, err := s.db.Get(blockID[:])
	if errors.Is(err, database.ErrNotFound) {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	// Get block

	v, err = s.db.Get(v)
	if errors.Is(err, database.ErrNotFound) {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (s *StandardDBIndexer) BlockAlreadyIndexed(height uint64) bool {
	k := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(k, height)
	exists, _ := s.db.Has(k)
	return exists
}

/*
Helper storage functions
*/

func (*StandardDBIndexer) storeTransaction(
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

func (*StandardDBIndexer) storeTransactionStateful(
	batch database.KeyValueWriter,
	txID ids.ID,
	timestamp int64,
) error {
	v := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(timestamp))
	return batch.Put(txID[:], v)
}

func (*StandardDBIndexer) mapBlockIDToHeight(
	batch database.KeyValueWriter,
	blockID ids.ID,
	blockHeight uint64,
) error {
	v := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(v, blockHeight)
	return batch.Put(blockID[:], v)
}

func (*StandardDBIndexer) storeBlock(
	batch database.KeyValueWriter,
	blk *chain.StatelessBlock,
) error {
	k := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(k, blk.Hght)
	v := blk.Bytes()
	return batch.Put(k, v)
}

func (*StandardDBIndexer) storeStatefulBlock(
	batch database.KeyValueWriter,
	blk *chain.StatefulBlock,
) error {
	k := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(k, blk.Hght)
	v, err := blk.Marshal()
	if err != nil {
		return err
	}
	return batch.Put(k, v)
}
