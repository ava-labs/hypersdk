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
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

// TODO: implement database cleanup
var errBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB            *pebble.Database
	REMOVEME_blockIDDB *pebble.Database
	txLookupDB         *pebble.Database
	lastHeight         atomic.Uint64
	parser             chain.Parser
}

func NewIndexer(path string, parser chain.Parser) (*Indexer, error) {
	blockDB, _, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create block DB: %w", err)
	}

	REMOVEME_blockIDDB, _, err := pebble.New(filepath.Join(path, "blockID"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create blockID DB: %w", err)
	}

	txLookupDB, _, err := pebble.New(filepath.Join(path, "txLookup"), pebble.NewDefaultConfig())
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
		txLookupDB:         txLookupDB,
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
	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	//TODO: batch

	if err := i.blockDB.Put(packUint64(blk.Block.Hght), executedBlkBytes); err != nil {
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
	batch := i.txLookupDB.NewBatch()
	defer batch.Reset()

	for j, tx := range blk.Block.Txs {
		txID := tx.ID()
		txLookupInfo := append(packUint64(blk.Block.Hght), packUint32(uint32(j))...)
		if err := batch.Put(txID[:], txLookupInfo); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (i *Indexer) GetTransaction(txID ids.ID) (*chain.Transaction, *chain.Result, error) {
	txLookupBytes, err := i.txLookupDB.Get(txID[:])
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, nil, ErrTxNotFound
		}
		return nil, nil, err
	}

	if len(txLookupBytes) != consts.Uint64Len+consts.Uint32Len {
		return nil, nil, fmt.Errorf("invalid tx lookup info")
	}

	blockHeight := unpackUint64(txLookupBytes[:consts.Uint64Len])
	txIndex := unpackUint32(txLookupBytes[consts.Uint64Len:])

	executedBlock, err := i.GetBlockByHeight(blockHeight)
	if err != nil {
		return nil, nil, err
	}

	if int(txIndex) >= len(executedBlock.Block.Txs) {
		return nil, nil, fmt.Errorf("tx index out of range")
	}

	tx := executedBlock.Block.Txs[txIndex]
	result := executedBlock.Results[txIndex]

	return tx, result, nil
}

func (i *Indexer) Close() error {
	errs := wrappers.Errs{}
	errs.Add(
		i.txLookupDB.Close(),
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

func packUint32(v uint32) []byte {
	b := make([]byte, consts.Uint32Len)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func unpackUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}
