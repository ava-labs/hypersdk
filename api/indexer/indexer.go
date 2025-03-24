// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

const (
	maxBlockWindow uint64 = 1_000_000
	blockEntryByte        = iota + 1
)

var blockEntryKeyPrefix = []byte{blockEntryByte}

var (
	errBlockNotFound           = errors.New("block not found")
	errTxResultNotFound        = errors.New("transaction result not found")
	errInternalIndexerMismatch = errors.New("internal indexer mismatch")
	errZeroBlockWindow         = errors.New("indexer configuration of non zero block window is expected")
	errInvalidBlockWindowSize  = errors.New("invalid indexer block window size specified")
)

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB     *pebble.Database // height -> block bytes
	blockWindow uint64           // Maximum window of blocks to retain
	parser      chain.Parser

	// synchronization mutex between r/w
	mu                 sync.RWMutex
	blockIDToHeight    map[ids.ID]uint64
	blockHeightToBlock map[uint64]*chain.ExecutedBlock
	txCache            map[ids.ID]cachedTransaction
	lastHeight         uint64
}

type cachedTransaction struct {
	// blkHeight is the block height that contains the transaction
	blkHeight uint64
	// index is the position where the transaction appear within the block.
	index int
}

func NewIndexer(path string, parser chain.Parser, blockWindow uint64) (*Indexer, error) {
	switch {
	case blockWindow > maxBlockWindow:
		return nil, fmt.Errorf("%w: block window %d exceeds maximum %d", errInvalidBlockWindowSize, blockWindow, maxBlockWindow)
	case blockWindow == 0:
		return nil, errZeroBlockWindow
	}
	blockDB, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig(), prometheus.NewRegistry())
	if err != nil {
		return nil, err
	}

	i := &Indexer{
		blockDB:            blockDB,
		blockIDToHeight:    make(map[ids.ID]uint64, int(blockWindow)),
		blockHeightToBlock: make(map[uint64]*chain.ExecutedBlock, int(blockWindow)),
		txCache:            make(map[ids.ID]cachedTransaction),
		blockWindow:        blockWindow,
		parser:             parser,
		lastHeight:         math.MaxUint64,
	}

	return i, i.initBlocks()
}

func (i *Indexer) initBlocks() error {
	// Load blockID <-> height mapping
	iter := i.blockDB.NewIteratorWithPrefix(blockEntryKeyPrefix)
	defer iter.Release()

	for iter.Next() {
		value := iter.Value()
		blk, err := chain.UnmarshalExecutedBlock(value, i.parser)
		if err != nil {
			return err
		}
		i.insertBlockIntoCache(blk)
	}
	if err := iter.Error(); err != nil {
		return err
	}
	iter.Release()

	if i.lastHeight > i.blockWindow {
		lastRetainedHeight := i.lastHeight - i.blockWindow
		lastRetainedHeightKey := blockEntryKey(lastRetainedHeight)
		firstBlkKey := blockEntryKey(0)
		if err := i.blockDB.DeleteRange(firstBlkKey, lastRetainedHeightKey); err != nil {
			return err
		}
	}
	return nil
}

func (i *Indexer) Notify(_ context.Context, blk *chain.ExecutedBlock) error {
	i.mu.Lock()
	i.insertBlockIntoCache(blk)
	i.mu.Unlock()

	return i.storeBlock(blk)
}

// insertBlockIntoCache add the given block and its transactions to the
// cache.
// assumes the write lock is held
func (i *Indexer) insertBlockIntoCache(blk *chain.ExecutedBlock) {
	if evictedBlk, ok := i.blockHeightToBlock[blk.Block.Hght-i.blockWindow]; ok {
		// remove the block from the caches
		delete(i.blockIDToHeight, evictedBlk.Block.GetID())
		delete(i.blockHeightToBlock, evictedBlk.Block.GetHeight())

		// remove the transactions from the cache.
		for _, tx := range evictedBlk.Block.Txs {
			delete(i.txCache, tx.GetID())
		}
	}

	i.blockIDToHeight[blk.Block.GetID()] = blk.Block.Hght
	i.blockHeightToBlock[blk.Block.Hght] = blk

	for idx, tx := range blk.Block.Txs {
		i.txCache[tx.GetID()] = cachedTransaction{
			blkHeight: blk.Block.Hght,
			index:     idx,
		}
	}
	i.lastHeight = blk.Block.Hght
}

// storeBlock persist the given block to the database, and deletes a block
// if it surpasses the retention window
func (i *Indexer) storeBlock(blk *chain.ExecutedBlock) error {
	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	blkBatch := i.blockDB.NewBatch()

	if err := blkBatch.Put(blockEntryKey(blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}

	if err := blkBatch.Delete(blockEntryKey(blk.Block.Hght - i.blockWindow)); err != nil {
		return err
	}

	return blkBatch.Write()
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.lastHeight == math.MaxUint64 {
		return nil, database.ErrNotFound
	}
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

func (i *Indexer) GetTransaction(txID ids.ID) (bool, *chain.Transaction, int64, *chain.Result, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	cachedTx, ok := i.txCache[txID]
	if !ok {
		return false, nil, 0, nil, nil
	}
	blk, ok := i.blockHeightToBlock[cachedTx.blkHeight]
	if !ok {
		return false, nil, 0, nil, nil
	}
	if len(blk.Block.Txs) <= cachedTx.index {
		// this should never happen, regardless of input parameters, since the indexer is atomically populating
		// the txCache and blockHeightToBlock with the transaction index corresponding to the same transaction.
		return false, nil, 0, nil, fmt.Errorf("%w: block height %d, transaction index %d", errInternalIndexerMismatch, blk.Block.Hght, cachedTx.index)
	}
	if len(blk.ExecutionResults.Results) <= cachedTx.index {
		return false, nil, 0, nil, fmt.Errorf("%w: block height %d, transaction index %d", errTxResultNotFound, blk.Block.Hght, cachedTx.index)
	}
	tx := blk.Block.Txs[cachedTx.index]
	result := blk.ExecutionResults.Results[cachedTx.index]
	return true, tx, blk.Block.Tmstmp, result, nil
}

func (i *Indexer) Close() error {
	return i.blockDB.Close()
}

func blockEntryKey(height uint64) []byte {
	return binary.BigEndian.AppendUint64(blockEntryKeyPrefix, height)
}
