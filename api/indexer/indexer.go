// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

const maxBlockWindow uint64 = 1_000_000

var errBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB            *pebble.Database
	REMOVEME_blockIDDB *pebble.Database
	REMOVEME_txDB      *pebble.Database
	blockWindow        uint64 // Maximum window of blocks to retain
	lastHeight         atomic.Uint64
	parser             chain.Parser
}

func NewIndexer(path string, parser chain.Parser, blockWindow uint64) (*Indexer, error) {
	if blockWindow > maxBlockWindow {
		return nil, fmt.Errorf("block window %d exceeds maximum %d", blockWindow, maxBlockWindow)
	}

	blockDB, _, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create block DB: %w", err)
	}

	REMOVEME_blockIDDB, _, err := pebble.New(filepath.Join(path, "blockID"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create blockID DB: %w", err)
	}

	REMOVEME_txDB, _, err := pebble.New(filepath.Join(path, "tx"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create tx DB: %w", err)
	}

	lastHeightValue := uint64(0)
	lastHeightBytes, err := REMOVEME_blockIDDB.Get([]byte("latest"))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("failed to get latest block height: %w", err)
	}
	if err == nil {
		lastHeightValue = unpackUint64(lastHeightBytes)
	}

	indexer := &Indexer{
		blockDB:            blockDB,
		REMOVEME_blockIDDB: REMOVEME_blockIDDB,
		REMOVEME_txDB:      REMOVEME_txDB,
		blockWindow:        blockWindow,
		parser:             parser,
	}
	indexer.lastHeight.Store(lastHeightValue)

	return indexer, nil
}

func (i *Indexer) Accept(blk *chain.ExecutedBlock) error {
	if err := i.storeTransactions(blk); err != nil {
		return err
	}
	return i.storeBlock(blk)
}

func (i *Indexer) storeBlock(blk *chain.ExecutedBlock) error {
	if i.blockWindow == 0 {
		return nil
	}

	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	//TODO: batch

	if err := i.blockDB.Put(packUint64(blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}
	// Ignore overflows in key calculation which will simply delete a non-existent key
	if err := i.blockDB.Delete(packUint64(blk.Block.Hght - i.blockWindow)); err != nil {
		return err
	}

	if err := i.REMOVEME_blockIDDB.Put(blk.BlockID[:], packUint64(blk.Block.Hght)); err != nil {
		return err
	}

	if err := i.REMOVEME_blockIDDB.Put([]byte("latest"), packUint64(blk.Block.Hght)); err != nil {
		return err
	}

	i.lastHeight.Store(blk.Block.Hght)

	return nil
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	height := i.lastHeight.Load()
	return i.GetBlockByHeight(height)
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blkBytes, err := i.blockDB.Get(packUint64(height))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, errBlockNotFound
		}
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(blkBytes, i.parser)
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	heightBytes, err := i.REMOVEME_blockIDDB.Get(blkID[:])
	if err != nil {
		return nil, err
	}
	height := unpackUint64(heightBytes)
	return i.GetBlockByHeight(height)
}

func (i *Indexer) storeTransactions(blk *chain.ExecutedBlock) error {
	batch := i.REMOVEME_txDB.NewBatch()
	defer batch.Reset()

	for j, tx := range blk.Block.Txs {
		result := blk.Results[j]
		if err := i.storeTransaction(
			batch,
			tx.ID(),
			blk.Block.Tmstmp,
			result.Success,
			result.Units,
			result.Fee,
			result.Outputs,
		); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (*Indexer) storeTransaction(
	batch database.KeyValueWriter,
	txID ids.ID,
	timestamp int64,
	success bool,
	units fees.Dimensions,
	fee uint64,
	outputs [][]byte,
) error {
	outputLength := consts.ByteLen // Single byte containing number of outputs
	for _, output := range outputs {
		outputLength += consts.Uint32Len + len(output)
	}
	txResultLength := consts.Uint64Len + consts.BoolLen + fees.DimensionsLen + consts.Uint64Len + outputLength

	writer := codec.NewWriter(txResultLength, consts.NetworkSizeLimit)
	writer.PackUint64(uint64(timestamp))
	writer.PackBool(success)
	writer.PackFixedBytes(units.Bytes())
	writer.PackUint64(fee)
	writer.PackByte(byte(len(outputs)))
	for _, output := range outputs {
		writer.PackBytes(output)
	}
	if err := writer.Err(); err != nil {
		return err
	}
	return batch.Put(txID[:], writer.Bytes())
}

func (i *Indexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, [][]byte, error) {
	v, err := i.REMOVEME_txDB.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, fees.Dimensions{}, 0, nil, nil
	}
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	reader := codec.NewReader(v, consts.NetworkSizeLimit)
	timestamp := reader.UnpackUint64(true)
	success := reader.UnpackBool()
	dimensionsBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &dimensionsBytes)
	fee := reader.UnpackUint64(true)
	numOutputs := int(reader.UnpackByte())
	outputs := make([][]byte, numOutputs)
	for i := range outputs {
		outputs[i] = reader.UnpackLimitedBytes(consts.NetworkSizeLimit)
	}
	if err := reader.Err(); err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	dimensions, err := fees.UnpackDimensions(dimensionsBytes)
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	return true, int64(timestamp), success, dimensions, fee, outputs, nil
}

func (i *Indexer) Close() error {
	errs := wrappers.Errs{}
	errs.Add(
		i.REMOVEME_txDB.Close(),
		i.blockDB.Close(),
		i.REMOVEME_blockIDDB.Close(),
	)
	return errs.Err
}

func packUint64(v uint64) []byte {
	b := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func unpackUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
