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

const maxBlockWindow uint64 = 1_000_000

const (
	blockEntryByte = iota + 1
	latestBlockHeightByte
)

var (
	blockEntryKeyPrefix  = []byte{blockEntryByte}
	latestBlockHeightKey = []byte{latestBlockHeightByte}
)

var (
	errBlockNotFound          = errors.New("block not found")
	errTxResultNotFound       = errors.New("transaction result not found")
	errZeroBlockWindow        = errors.New("indexer configuration of non zero block window is expected")
	errInvalidBlockWindowSize = errors.New("invalid indexer block window size specified")
)

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB            *pebble.Database // height -> block bytes
	cachedBlocks       []uint64
	blockIDToHeight    map[ids.ID]uint64
	blockHeightToBlock map[uint64]*chain.ExecutedBlock
	txCache            map[ids.ID]cachedTransaction
	blockWindow        uint64 // Maximum window of blocks to retain
	lastHeight         uint64
	parser             chain.Parser

	// synchronization mutex between r/w
	mu sync.RWMutex
}

type cachedTransaction struct {
	// blk is the block that contains the transaction
	blk *chain.ExecutedBlock
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
		cachedBlocks:       make([]uint64, 0, int(blockWindow)),
	}

	return i, i.initBlocks()
}

func (i *Indexer) initBlocks() error {
	// Return immediately if the db is empty
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
		i.insertBlockIntoCache(blk)
	}
	if err := iter.Error(); err != nil {
		return err
	}
	iter.Release()
	return nil
}

func (i *Indexer) Notify(_ context.Context, blk *chain.ExecutedBlock) error {
	i.mu.Lock()
	evictedBlockHeights := i.insertBlockIntoCache(blk)
	i.mu.Unlock()

	return i.storeBlock(blk, evictedBlockHeights)
}

// insertBlockIntoCache add the given block and its transactions to the
// cache. It returns a slice of the evicted block heights.
// assumes the write lock is held
func (i *Indexer) insertBlockIntoCache(blk *chain.ExecutedBlock) []uint64 {
	var evictedBlockHeights []uint64
	if len(i.cachedBlocks) == int(i.blockWindow) {
		evictedBlockHeights = []uint64{i.cachedBlocks[0]}
		i.cachedBlocks = i.cachedBlocks[1:]

		// find the block in the blocks cache.
		evictedBlk := i.blockHeightToBlock[evictedBlockHeights[0]]

		// remove the block from the caches
		delete(i.blockIDToHeight, evictedBlk.Block.GetID())
		delete(i.blockHeightToBlock, evictedBlockHeights[0])

		// remove the transactions from the cache.
		for _, tx := range evictedBlk.Block.Txs {
			delete(i.txCache, tx.GetID())
		}
	}
	i.cachedBlocks = append(i.cachedBlocks, blk.Block.Hght)
	i.blockIDToHeight[blk.Block.GetID()] = blk.Block.Hght
	i.blockHeightToBlock[blk.Block.Hght] = blk

	for idx, tx := range blk.Block.Txs {
		i.txCache[tx.GetID()] = cachedTransaction{
			blk:   blk,
			index: idx,
		}
	}
	i.lastHeight = blk.Block.Hght

	return evictedBlockHeights
}

// storeBlock persist the given block to the database, and remove the
// evicted blocks.
func (i *Indexer) storeBlock(blk *chain.ExecutedBlock, evictedBlockHeights []uint64) error {
	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	blkBatch := i.blockDB.NewBatch()

	if err := blkBatch.Put(blockEntryKey(blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}

	for _, evictedBlockHeight := range evictedBlockHeights {
		if err := blkBatch.Delete(blockEntryKey(evictedBlockHeight)); err != nil {
			return err
		}
	}

	if err := blkBatch.Put(latestBlockHeightKey, binary.BigEndian.AppendUint64(nil, i.lastHeight)); err != nil {
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
	tx := cachedTx.blk.Block.Txs[cachedTx.index]
	if len(cachedTx.blk.ExecutionResults.Results) <= cachedTx.index {
		return false, nil, 0, nil, fmt.Errorf("%w: block height %d, transaction index %d", errTxResultNotFound, cachedTx.blk.Block.Hght, cachedTx.index)
	}
	result := cachedTx.blk.ExecutionResults.Results[cachedTx.index]
	return true, tx, cachedTx.blk.Block.Tmstmp, result, nil
}

func (i *Indexer) Close() error {
	return i.blockDB.Close()
}

func blockEntryKey(height uint64) []byte {
	return binary.BigEndian.AppendUint64(blockEntryKeyPrefix, height)
}
