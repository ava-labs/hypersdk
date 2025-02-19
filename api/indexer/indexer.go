// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

const maxBlockWindow uint64 = 1_000_000

var errBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB            *pebble.Database // height -> block bytes
	blockIDToHeight    *cache.FIFO[ids.ID, uint64]
	blockHeightToBlock *cache.FIFO[uint64, *chain.ExecutedBlock]
	blockWindow        uint64 // Maximum window of blocks to retain
	lastHeight         uint64
	parser             chain.Parser

	// ID -> timestamp, success, units, fee, outputs
	txDB *pebble.Database
	// synchronization mutex between r/w
	mu sync.RWMutex
}

func NewIndexer(path string, parser chain.Parser, blockWindow uint64) (*Indexer, error) {
	if blockWindow > maxBlockWindow {
		return nil, fmt.Errorf("block window %d exceeds maximum %d", blockWindow, maxBlockWindow)
	}
	txDB, err := pebble.New(filepath.Join(path, "tx"), pebble.NewDefaultConfig(), prometheus.NewRegistry())
	if err != nil {
		return nil, err
	}
	blockDB, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig(), prometheus.NewRegistry())
	if err != nil {
		return nil, err
	}

	blockIDCache, err := cache.NewFIFO[ids.ID, uint64](int(blockWindow))
	if err != nil {
		return nil, err
	}
	blockCache, err := cache.NewFIFO[uint64, *chain.ExecutedBlock](int(blockWindow))
	if err != nil {
		return nil, err
	}
	i := &Indexer{
		blockDB:            blockDB,
		blockIDToHeight:    blockIDCache,
		blockHeightToBlock: blockCache,
		blockWindow:        blockWindow,
		parser:             parser,
		txDB:               txDB,
	}
	return i, i.initBlocks()
}

func (i *Indexer) initBlocks() error {
	// Load blockID <-> height mapping
	iter := i.blockDB.NewIterator()
	defer iter.Release()

	var lastHeight uint64
	for iter.Next() {
		value := iter.Value()
		blk, err := chain.UnmarshalExecutedBlock(value, i.parser)
		if err != nil {
			return err
		}

		i.blockIDToHeight.Put(blk.Block.GetID(), blk.Block.Hght)
		if _, removedBlk := i.blockHeightToBlock.Put(blk.Block.Hght, blk); removedBlk != nil {
			// the following should not happen, as we maintain only blockWindow entries; however,
			// if it does, the following would correct that.
			err = i.blockDB.Delete(binary.BigEndian.AppendUint64(nil, *removedBlk))
			if err != nil {
				return err
			}
		}
		lastHeight = blk.Block.Hght
	}
	if err := iter.Error(); err != nil {
		return err
	}
	iter.Release()

	i.lastHeight = lastHeight
	return nil
}

func (i *Indexer) Notify(_ context.Context, blk *chain.ExecutedBlock) error {
	i.mu.Lock()
	defer i.mu.Unlock()
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

	_, removedBlkHeight := i.blockHeightToBlock.Put(blk.Block.Hght, blk)

	blkBatch := i.blockDB.NewBatch()
	defer blkBatch.Reset()

	if err := blkBatch.Put(binary.BigEndian.AppendUint64(nil, blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}

	if removedBlkHeight != nil {
		if err := blkBatch.Delete(binary.BigEndian.AppendUint64(nil, *removedBlkHeight)); err != nil {
			return err
		}
	}

	i.blockIDToHeight.Put(blk.Block.GetID(), blk.Block.Hght)

	if i.lastHeight >= blk.Block.Hght {
		panic(errors.New("block height was not monolotically growing"))
	}
	// ensure that lastHeight always contains the highest height we've seen.
	i.lastHeight = max(i.lastHeight, blk.Block.Hght)
	return blkBatch.Write()
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.GetBlockByHeight(i.lastHeight)
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.getBlockByHeight(height)
}

func (i *Indexer) getBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blk, ok := i.blockHeightToBlock.Get(height)
	if !ok {
		return nil, fmt.Errorf("%w: height=%d", errBlockNotFound, height)
	}
	return blk, nil
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	height, ok := i.blockIDToHeight.Get(blkID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errBlockNotFound, blkID)
	}
	return i.getBlockByHeight(height)
}

func (i *Indexer) storeTransactions(blk *chain.ExecutedBlock) error {
	batch := i.txDB.NewBatch()
	defer batch.Reset()

	for j, tx := range blk.Block.Txs {
		result := blk.Results[j]
		if err := i.storeTransaction(
			batch,
			tx.GetID(),
			tx.Bytes(),
			blk.Block.Tmstmp,
			result.Success,
			result.Units,
			result.Fee,
			result.Outputs,
			string(result.Error),
		); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (*Indexer) storeTransaction(
	batch database.KeyValueWriter,
	txID ids.ID,
	txBytes []byte,
	timestamp int64,
	success bool,
	units fees.Dimensions,
	fee uint64,
	outputs [][]byte,
	errorStr string,
) error {
	storageTx := storageTx{
		Timestamp: timestamp,
		Success:   success,
		Units:     units.Bytes(),
		Fee:       fee,
		Outputs:   outputs,
		Error:     errorStr,
		TxBytes:   txBytes,
	}
	storageTxBytes := storageTx.MarshalCanoto()

	return batch.Put(txID[:], storageTxBytes)
}

func (i *Indexer) GetTransaction(txID ids.ID) (bool, *chain.Transaction, []byte, int64, *chain.Result, error) {
	v, err := i.txDB.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, nil, nil, 0, nil, nil
	}
	if err != nil {
		return false, nil, nil, 0, nil, err
	}

	storageTx := storageTx{}
	if err := storageTx.UnmarshalCanoto(v); err != nil {
		return false, nil, nil, 0, nil, fmt.Errorf("failed to unmarshal storage tx %s: %w", txID, err)
	}

	unpackedUnits, err := fees.UnpackDimensions(storageTx.Units)
	if err != nil {
		return false, nil, nil, 0, nil, fmt.Errorf("failed to unpack units for storage tx %s: %w", txID, err)
	}

	p := codec.NewReader(storageTx.TxBytes, consts.NetworkSizeLimit)
	tx, err := chain.UnmarshalTx(p, i.parser.ActionCodec(), i.parser.AuthCodec())
	if err != nil {
		return false, nil, nil, 0, nil, fmt.Errorf("failed to unmarshal tx %s: %w", txID, err)
	}
	result := &chain.Result{
		Success: storageTx.Success,
		Error:   []byte(storageTx.Error),
		Outputs: storageTx.Outputs,
		Units:   unpackedUnits,
		Fee:     storageTx.Fee,
	}
	return true, tx, storageTx.TxBytes, storageTx.Timestamp, result, nil
}

func (i *Indexer) Close() error {
	errs := wrappers.Errs{}
	errs.Add(
		i.txDB.Close(),
		i.blockDB.Close(),
	)
	return errs.Err
}
