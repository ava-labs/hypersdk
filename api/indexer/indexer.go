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
	lastHeight         atomic.Uint64
	parser             chain.Parser

	// ID -> timestamp, success, units, fee, outputs
	txDB *pebble.Database
}

func NewIndexer(path string, parser chain.Parser, blockWindow uint64) (*Indexer, error) {
	if blockWindow > maxBlockWindow {
		return nil, fmt.Errorf("block window %d exceeds maximum %d", blockWindow, maxBlockWindow)
	}
	txDB, _, err := pebble.New(filepath.Join(path, "tx"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	blockDB, _, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig())
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

		i.blockIDToHeight.Put(blk.BlockID, blk.Block.Hght)
		i.blockHeightToBlock.Put(blk.Block.Hght, blk)
		lastHeight = blk.Block.Hght
	}
	if err := iter.Error(); err != nil {
		return err
	}
	iter.Release()

	i.lastHeight.Store(lastHeight)

	if lastHeight > i.blockWindow {
		lastRetainedHeight := lastHeight - i.blockWindow
		lastRetainedHeightBytes := binary.BigEndian.AppendUint64(nil, lastRetainedHeight)
		if err := i.blockDB.DeleteRange(nil, lastRetainedHeightBytes); err != nil {
			return err
		}
	}

	return nil
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

	if err := i.blockDB.Put(binary.BigEndian.AppendUint64(nil, blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}
	// Ignore overflows in key calculation which will simply delete a non-existent key
	if err := i.blockDB.Delete(binary.BigEndian.AppendUint64(nil, blk.Block.Hght-i.blockWindow)); err != nil {
		return err
	}

	i.blockIDToHeight.Put(blk.BlockID, blk.Block.Hght)
	i.blockHeightToBlock.Put(blk.Block.Hght, blk)
	i.lastHeight.Store(blk.Block.Hght)

	return nil
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	return i.GetBlockByHeight(i.lastHeight.Load())
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blk, ok := i.blockHeightToBlock.Get(height)
	if !ok {
		return nil, fmt.Errorf("%w: height=%d", errBlockNotFound, height)
	}
	return blk, nil
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	height, ok := i.blockIDToHeight.Get(blkID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errBlockNotFound, blkID)
	}
	return i.GetBlockByHeight(height)
}

func (i *Indexer) storeTransactions(blk *chain.ExecutedBlock) error {
	batch := i.txDB.NewBatch()
	defer batch.Reset()

	for j, tx := range blk.Block.Txs {
		p := codec.NewWriter(tx.Size(), consts.NetworkSizeLimit)
		err := tx.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}

		result := blk.Results[j]
		err = result.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}

		if p.Err() != nil {
			return fmt.Errorf("failed to marshal transaction: %w", p.Err())
		}

		txID := tx.ID()
		err = batch.Put(txID[:], p.Bytes())
		if err != nil {
			return fmt.Errorf("failed to store transaction: %w", err)
		}
	}

	return batch.Write()
}

func (i *Indexer) GetTransaction(txID ids.ID) (bool, *chain.Transaction, *chain.Result, error) {
	v, err := i.txDB.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, nil, nil, nil
	}
	if err != nil {
		return false, nil, nil, err
	}
	reader := codec.NewReader(v, consts.NetworkSizeLimit)

	tx, err := chain.UnmarshalTx(reader, i.parser.ActionRegistry(), i.parser.AuthRegistry())
	if err != nil {
		return false, nil, nil, err
	}

	result, err := chain.UnmarshalResult(reader)
	if err != nil {
		return false, nil, nil, err
	}

	return true, tx, result, nil
}

func (i *Indexer) Close() error {
	errs := wrappers.Errs{}
	errs.Add(
		i.txDB.Close(),
		i.blockDB.Close(),
	)
	return errs.Err
}
