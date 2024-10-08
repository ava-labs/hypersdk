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
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

// TODO: implement database cleanup
var ErrBlockNotFound = errors.New("block not found")

var _ event.Subscription[*chain.ExecutedBlock] = (*Indexer)(nil)

type Indexer struct {
	blockDB         *pebble.Database
	blockIDLookupDB *pebble.Database
	txLookupDB      *pebble.Database
	parser          chain.Parser
}

func NewIndexer(path string, parser chain.Parser) (*Indexer, error) {
	blockDB, _, err := pebble.New(filepath.Join(path, "block"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create block DB: %w", err)
	}

	blockIDLookupDB, _, err := pebble.New(filepath.Join(path, "blockID"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create blockID DB: %w", err)
	}

	txLookupDB, _, err := pebble.New(filepath.Join(path, "txLookup"), pebble.NewDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create tx DB: %w", err)
	}

	indexer := &Indexer{
		blockDB:         blockDB,
		blockIDLookupDB: blockIDLookupDB,
		txLookupDB:      txLookupDB,
		parser:          parser,
	}

	return indexer, nil
}

func (i *Indexer) Accept(blk *chain.ExecutedBlock) error {
	errChan := make(chan error, 3)

	go func() { errChan <- i.saveTransactionIDs(blk) }()
	go func() { errChan <- i.saveBlock(blk) }()
	go func() { errChan <- i.saveBlockHeight(blk) }()

	for j := 0; j < 3; j++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (i *Indexer) saveTransactionIDs(blk *chain.ExecutedBlock) error {
	//save trasnsaction IDs
	txBatch := i.txLookupDB.NewBatch()
	defer txBatch.Reset()

	for j, tx := range blk.Block.Txs {
		txID := tx.ID()
		txLookupInfo := append(packUint64(blk.Block.Hght), packUint32(uint32(j))...)
		if err := txBatch.Put(txID[:], txLookupInfo); err != nil {
			return err
		}
	}

	if err := txBatch.Write(); err != nil {
		return err
	}

	return nil
}

func (i *Indexer) saveBlock(blk *chain.ExecutedBlock) error {
	blockBatch := i.blockDB.NewBatch()
	defer blockBatch.Reset()

	executedBlkBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	if err := blockBatch.Put(packUint64(blk.Block.Hght), executedBlkBytes); err != nil {
		return err
	}

	if err := blockBatch.Write(); err != nil {
		return err
	}

	return nil
}

func (i *Indexer) saveBlockHeight(blk *chain.ExecutedBlock) error {

	//save block height
	blockIdBatch := i.blockIDLookupDB.NewBatch()

	if err := blockIdBatch.Put(blk.BlockID[:], packUint64(blk.Block.Hght)); err != nil {
		return err
	}

	if err := blockIdBatch.Put([]byte("latest"), packUint64(blk.Block.Hght)); err != nil {
		return err
	}

	if err := blockIdBatch.Write(); err != nil {
		return err
	}

	return nil
}

func (i *Indexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	lastHeightValue := uint64(0)
	lastHeightBytes, err := i.blockIDLookupDB.Get([]byte("latest"))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("failed to get latest block height: %w", err)
	}
	if err == nil {
		lastHeightValue = unpackUint64(lastHeightBytes)
	}

	return i.GetBlockByHeight(lastHeightValue)
}

func (i *Indexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	blkBytes, err := i.blockDB.Get(packUint64(height))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(blkBytes, i.parser)
}

func (i *Indexer) GetBlock(blkID ids.ID) (*chain.ExecutedBlock, error) {
	heightBytes, err := i.blockIDLookupDB.Get(blkID[:])
	if err != nil {
		return nil, err
	}
	height := unpackUint64(heightBytes)
	return i.GetBlockByHeight(height)
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
		i.blockIDLookupDB.Close(),
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
