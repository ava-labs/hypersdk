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
	_ AcceptedSubscriber = (*DBIndexer)(nil)
	_ AcceptedSubscriber = (*NoOpIndexer)(nil)

	_ Indexer = (*DBIndexer)(nil)

	failureByte = byte(0x0)
	successByte = byte(0x1)
)

type Indexer interface {
	AcceptedSubscriber
	GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error)
	AcceptedStateful(context.Context, *chain.StatefulBlock, []*chain.Result) error
	GetBlockByHeight(height uint64) ([]byte, error)
	GetBlock(blockID ids.ID) ([]byte, error)
}

type DBIndexer struct {
	db database.Database
}

func NewDBIndexer(db database.Database) *DBIndexer {
	return &DBIndexer{db: db}
}

func (s *DBIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
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

func (s *DBIndexer) Accepted(_ context.Context, blk *chain.StatelessBlock) error {
	return s.innerAccepted(blk.StatefulBlock, blk.Results())
}

func (s *DBIndexer) AcceptedStateful(_ context.Context, blk *chain.StatefulBlock, results []*chain.Result) error {
	return s.innerAccepted(blk, results)
}

func (s *DBIndexer) innerAccepted(blk *chain.StatefulBlock, results []*chain.Result) error {
	batch := s.db.NewBatch()
	defer batch.Reset()

	// Store block bytes
	if err := s.putStatefulBlock(batch, blk); err != nil {
		return err
	}

	blkID, err := blk.ID()
	if err != nil {
		return err
	}

	// Map block ID to block height
	if err := s.putBlockID(batch, blkID, blk.Hght); err != nil {
		return err
	}
	// Store TX
	timestamp := blk.Tmstmp
	for j, tx := range blk.Txs {
		result := results[j]
		if err := s.putTransaction(
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

func (s *DBIndexer) GetBlockByHeight(height uint64) ([]byte, error) {
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

func (s *DBIndexer) GetBlock(blockID ids.ID) ([]byte, error) {
	// Get height
	v, err := s.db.Get(blockID[:])
	if errors.Is(err, database.ErrNotFound) {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}

	v, err = s.db.Get(v)
	if errors.Is(err, database.ErrNotFound) {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Maps block ID to height
func (*DBIndexer) putBlockID(
	batch database.KeyValueWriter,
	blockID ids.ID,
	blockHeight uint64,
) error {
	v := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(v, blockHeight)
	return batch.Put(blockID[:], v)
}

func (*DBIndexer) putStatefulBlock(
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

func (*DBIndexer) putTransaction(
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

type NoOpIndexer struct{}

func NewNoOpIndexer() *NoOpIndexer {
	return &NoOpIndexer{}
}

func (*NoOpIndexer) Accepted(_ context.Context, _ *chain.StatelessBlock) error {
	return nil
}

func (*NoOpIndexer) GetTransaction(_ ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return false, 0, false, fees.Dimensions{}, 0, errors.New("indexer not enabled")
}

func (*NoOpIndexer) GetBlock(_ ids.ID) ([]byte, error) {
	return nil, errors.New("indexer not enabled")
}

func (*NoOpIndexer) GetBlockByHeight(_ uint64) ([]byte, error) {
	return nil, errors.New("indexer not enabled")
}

func (*NoOpIndexer) AcceptedStateful(_ context.Context, _ *chain.StatefulBlock, _ []*chain.Result) error {
	return errors.New("indexer not enabled")
}
