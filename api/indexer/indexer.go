// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

// TODO: implement database cleanup
var ErrBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB     *pebble.Database
	parser      chain.Parser
	blockWindow uint64
}

func NewIndexer(path string, parser chain.Parser, blockWindow uint64) (*Indexer, error) {
	blockDB, _, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create block DB: %w", err)
	}

	indexer := &Indexer{
		blockDB:     blockDB,
		parser:      parser,
		blockWindow: blockWindow,
	}

	return indexer, nil
}
func (i *Indexer) Accept(blk *chain.ExecutedBlock) error {
	batch := i.blockDB.NewBatch()
	defer batch.Reset()

	// Save transaction IDs
	for j, tx := range blk.Block.Txs {
		txID := tx.ID()
		txLookupInfo := append(packUint64(blk.Block.Hght), packUint32(uint32(j))...)
		if err := batch.Put(txID[:], txLookupInfo); err != nil {
			return fmt.Errorf("failed to save transaction ID: %w", err)
		}
	}

	// Save block
	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal executed block: %w", err)
	}
	if err := batch.Put(blockByHeightKey(blk.Block.Hght), executedBlkBytes); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Save block height
	if err := batch.Put(blockHeightByIDKey(blk.BlockID), packUint64(blk.Block.Hght)); err != nil {
		return fmt.Errorf("failed to save block ID: %w", err)
	}
	if err := batch.Put([]byte("latest"), packUint64(blk.Block.Hght)); err != nil {
		return fmt.Errorf("failed to update latest block height: %w", err)
	}

	// Update earliest block if necessary
	earliestBytes, err := i.blockDB.Get([]byte("earliest"))
	earliestHeight := uint64(0)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// If earliest doesn't exist, this is the first block
			if err := batch.Put([]byte("earliest"), packUint64(blk.Block.Hght)); err != nil {
				return fmt.Errorf("failed to set earliest block height: %w", err)
			}
			earliestBytes = packUint64(blk.Block.Hght)
		} else {
			return fmt.Errorf("failed to get earliest block height: %w", err)
		}
	}

	earliestHeight = unpackUint64(earliestBytes)

	// Delete old blocks
	if blk.Block.Hght >= i.blockWindow {
		for earliestHeight <= blk.Block.Hght-i.blockWindow {
			oldBlockHeightKey := blockByHeightKey(earliestHeight)

			// Retrieve old block
			oldBlockBytes, err := i.blockDB.Get(oldBlockHeightKey)
			if err != nil {
				if !errors.Is(err, database.ErrNotFound) {
					return fmt.Errorf("failed to retrieve old block: %w", err)
				}
			} else {
				oldBlock, err := chain.UnmarshalExecutedBlock(oldBlockBytes, i.parser)
				if err != nil {
					return fmt.Errorf("failed to unmarshal old block: %w", err)
				}

				// Remove old block from blockDB
				if err := batch.Delete(oldBlockHeightKey); err != nil {
					return fmt.Errorf("failed to delete old block: %w", err)
				}

				// Remove old block ID from blockIDLookupDB
				if err := batch.Delete(blockHeightByIDKey(oldBlock.BlockID)); err != nil {
					return fmt.Errorf("failed to delete old block ID: %w", err)
				}

				// Remove old transactions from txLookupDB
				for _, tx := range oldBlock.Block.Txs {
					txID := tx.ID()
					if err := batch.Delete(txID[:]); err != nil {
						return fmt.Errorf("failed to delete old transaction: %w", err)
					}
				}
			}

			earliestHeight++
		}

		// Update earliest block height
		if err := batch.Put([]byte("earliest"), packUint64(earliestHeight)); err != nil {
			return fmt.Errorf("failed to update earliest block height: %w", err)
		}
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write batch: %w", err)
	}

	return nil
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	lastHeightValue := uint64(0)
	lastHeightBytes, err := i.blockDB.Get([]byte("latest"))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("failed to get latest block height: %w", err)
	}
	if err == nil {
		lastHeightValue = unpackUint64(lastHeightBytes)
	}

	return i.GetBlockByHeight(lastHeightValue)
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blkBytes, err := i.blockDB.Get(blockByHeightKey(height))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(blkBytes, i.parser)
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	heightBytes, err := i.blockDB.Get(blockHeightByIDKey(blkID))
	if err != nil {
		return nil, err
	}
	height := unpackUint64(heightBytes)
	return i.GetBlockByHeight(height)
}

func (i *Indexer) GetTransaction(txID ids.ID) (*chain.Transaction, *chain.Result, error) {
	txLookupBytes, err := i.blockDB.Get(txID[:])
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
	return i.blockDB.Close()
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

func blockByHeightKey(height uint64) []byte {
	return append([]byte{0}, packUint64(height)...)
}

func blockHeightByIDKey(blkOrTxID ids.ID) []byte {
	return append([]byte{1}, blkOrTxID[:]...)
}
