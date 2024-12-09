// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainstore

import (
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	blockPrefix         byte = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix byte = 0x1 // ID -> Height
	blockHeightIDPrefix byte = 0x2 // Height -> ID (don't always need full block from disk)
	lastAcceptedByte    byte = 0x3 // lastAcceptedByte -> lastAcceptedHeight
)

// ChainStore provides a persistent store that maps:
// blockHeight -> blockBytes
// blockID -> blockHeight
// blockHeight -> blockID
// TODO: add metrics / span tracing
type ChainStore struct {
	db database.Database
}

func New(db database.Database) *ChainStore {
	return &ChainStore{db: db}
}

func (c *ChainStore) GetLastAcceptedHeight() (uint64, error) {
	lastAcceptedHeightBytes, err := c.db.Get([]byte{lastAcceptedByte})
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(lastAcceptedHeightBytes)
}

func (c *ChainStore) UpdateLastAccepted(blkID ids.ID, height uint64, blockBytes []byte) error {
	batch := c.db.NewBatch()

	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	return errors.Join(
		batch.Put([]byte{lastAcceptedByte}, heightBytes),
		batch.Put(PrefixBlockIDHeightKey(blkID), heightBytes),
		batch.Put(PrefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(PrefixBlockKey(height), blockBytes),
		batch.Write(),
	)
}

func (c *ChainStore) GetBlock(blkID ids.ID) ([]byte, error) {
	height, err := c.GetBlockIDHeight(blkID)
	if err != nil {
		return nil, err
	}
	return c.GetBlockByHeight(height)
}

func (c *ChainStore) GetBlockIDAtHeight(blkHeight uint64) (ids.ID, error) {
	blkIDBytes, err := c.db.Get(PrefixBlockHeightIDKey(blkHeight))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(blkIDBytes), nil
}

func (c *ChainStore) GetBlockIDHeight(blkID ids.ID) (uint64, error) {
	blkHeightBytes, err := c.db.Get(PrefixBlockIDHeightKey(blkID))
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(blkHeightBytes)
}

func (c *ChainStore) GetBlockByHeight(blkHeight uint64) ([]byte, error) {
	return c.db.Get(PrefixBlockKey(blkHeight))
}

func PrefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func PrefixBlockIDHeightKey(id ids.ID) []byte {
	k := make([]byte, 1+ids.IDLen)
	k[0] = blockIDHeightPrefix
	copy(k[1:], id[:])
	return k
}

func PrefixBlockHeightIDKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}
