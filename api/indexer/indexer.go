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
	"github.com/ava-labs/avalanchego/utils/buffer"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

const maxBlockWindow uint64 = 1_000_000

var (
	blockEntryKeyPrefix  = []byte("block")
	latestBlockHeightKey = []byte("latestblockheight")
)

var errBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB            *pebble.Database // height -> block bytes
	cachedBlocks       buffer.Queue[uint64]
	blockIDToHeight    map[ids.ID]uint64
	blockHeightToBlock map[uint64]*chain.ExecutedBlock
	evictedBlocks      []uint64
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

	blockIDCache := make(map[ids.ID]uint64, int(blockWindow))
	blockHeightCache := make(map[uint64]*chain.ExecutedBlock, int(blockWindow))

	i := &Indexer{
		blockDB:            blockDB,
		blockIDToHeight:    blockIDCache,
		blockHeightToBlock: blockHeightCache,
		blockWindow:        blockWindow,
		parser:             parser,
		txDB:               txDB,
	}

	i.cachedBlocks, err = buffer.NewBoundedQueue(int(blockWindow), i.evictBlock)
	if err != nil {
		return nil, err
	}

	return i, i.initBlocks()
}

func (i *Indexer) evictBlock(blockHeight uint64) {
	// find the block in the blocks cache.
	blk := i.blockHeightToBlock[blockHeight]

	// remove the block from the caches
	delete(i.blockIDToHeight, blk.Block.GetID())
	delete(i.blockHeightToBlock, blockHeight)

	// add the block height to evictedBlocks so that it would get removed from database
	i.evictedBlocks = append(i.evictedBlocks, blockHeight)
}

func (i *Indexer) initBlocks() error {
	// is this a new database or an old one ?
	hasLastHeight, err := i.blockDB.Has(latestBlockHeightKey)
	if err != nil || !hasLastHeight {
		return err
	}

	lastHeight, err := i.blockDB.Get(latestBlockHeightKey)
	if err != nil {
		return err
	}

	i.lastHeight = binary.BigEndian.Uint64(lastHeight)

	if i.lastHeight > i.blockWindow {
		lastRetainedHeight := i.lastHeight - i.blockWindow
		lastRetainedHeightKey := blockEntryKey(lastRetainedHeight)
		firstBlkKey := blockEntryKey(0)
		if err := i.blockDB.DeleteRange(firstBlkKey, lastRetainedHeightKey); err != nil {
			return err
		}
	}

	// Load blockID <-> height mapping
	iter := i.blockDB.NewIteratorWithPrefix(blockEntryKeyPrefix)
	defer iter.Release()

	for iter.Next() {
		value := iter.Value()
		blk, err := chain.UnmarshalExecutedBlock(value, i.parser)
		if err != nil {
			return err
		}

		i.cachedBlocks.Push(blk.Block.Hght)
		i.blockIDToHeight[blk.Block.GetID()] = blk.Block.Hght
		i.blockHeightToBlock[blk.Block.Hght] = blk

	}
	if err := iter.Error(); err != nil {
		return err
	}
	iter.Release()

	return nil
}

func (i *Indexer) Notify(_ context.Context, blk *chain.ExecutedBlock) error {
	if i.blockWindow == 0 {
		return nil
	}
	if err := i.storeTransactions(blk); err != nil {
		return err
	}
	i.updateCachedBlocks(blk)
	return i.storeBlock(blk)
}

func (i *Indexer) updateCachedBlocks(blk *chain.ExecutedBlock) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cachedBlocks.Push(blk.Block.Hght)
	i.blockIDToHeight[blk.Block.GetID()] = blk.Block.Hght
	i.blockHeightToBlock[blk.Block.Hght] = blk
	i.lastHeight = blk.Block.Hght
}

func (i *Indexer) storeBlock(blk *chain.ExecutedBlock) error {
	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	blkBatch := i.blockDB.NewBatch()
	defer blkBatch.Reset()

	if err := blkBatch.Put(blockEntryKey(blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}

	for _, evictedBlockHeight := range i.evictedBlocks {
		if err := blkBatch.Delete(blockEntryKey(evictedBlockHeight)); err != nil {
			return err
		}
	}

	if err := blkBatch.Put(latestBlockHeightKey, binary.BigEndian.AppendUint64(nil, i.lastHeight)); err != nil {
		return err
	}

	// clear the slice without allocation
	i.evictedBlocks = i.evictedBlocks[:0]

	return blkBatch.Write()
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.getBlockByHeight(i.lastHeight)
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.getBlockByHeight(height)
}

func (i *Indexer) getBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blk, ok := i.blockHeightToBlock[height]
	if !ok {
		return nil, fmt.Errorf("%w: height=%d", errBlockNotFound, height)
	}
	return blk, nil
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	height, ok := i.blockIDToHeight[blkID]
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

func blockEntryKey(height uint64) []byte {
	return append(blockEntryKeyPrefix, binary.BigEndian.AppendUint64(nil, height)...)
}
