// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"math/rand"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	blockPrefix         byte = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	resultPrefix        byte = 0x1
	blockIDHeightPrefix byte = 0x2 // ID -> Height
	blockHeightIDPrefix byte = 0x3 // Height -> ID (don't always need full block from disk)
	lastAcceptedByte    byte = 0x4 // lastAcceptedByte -> lastAcceptedHeight
)

type Config struct {
	AcceptedBlockWindow      uint64 `json:"acceptedBlockWindow"`
	BlockCompactionAverageFrequency int    `json:"blockCompactionFrequency"`
}

func NewDefaultConfig() Config {
	return Config{
		AcceptedBlockWindow:      50_000, // ~3.5hr with 250ms block time (100GB at 2MB)
		BlockCompactionAverageFrequency: 32,     // 64 MB of deletion if 2 MB blocks
	}
}

// ChainStore provides a persistent store that maps:
// blockHeight -> blockBytes
// blockID -> blockHeight
// blockHeight -> blockID
// TODO: add metrics / span tracing
type ChainStore struct {
	config  Config
	log     logging.Logger
	db      database.Database
	metrics *metrics
}

func New(config Config, log logging.Logger, registry prometheus.Registerer, db database.Database) (*ChainStore, error) {
	metrics, err := newMetrics(registry)
	if err != nil {
		return nil, err
	}
	return &ChainStore{
		config:  config,
		log:     log,
		db:      db,
		metrics: metrics,
	}, nil
}

func (c *ChainStore) GetLastAcceptedHeight() (uint64, error) {
	lastAcceptedHeightBytes, err := c.db.Get([]byte{lastAcceptedByte})
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(lastAcceptedHeightBytes)
}

func (c *ChainStore) UpdateLastAccepted(blkID ids.ID, height uint64, blockBytes []byte, resultBytes []byte) error {
	batch := c.db.NewBatch()

	// TODO: add expiry
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	err := errors.Join(
		batch.Put([]byte{lastAcceptedByte}, heightBytes),
		batch.Put(PrefixBlockIDHeightKey(blkID), heightBytes),
		batch.Put(PrefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(PrefixBlockKey(height), blockBytes),
		batch.Put(PrefixResultKey(height), resultBytes),
	)
	if err != nil {
		return err
	}
	expiryHeight := height - c.config.AcceptedBlockWindow
	var expired bool
	if expiryHeight > 0 && expiryHeight < height { // ensure we don't free genesis
		if err := batch.Delete(PrefixBlockKey(expiryHeight)); err != nil {
			return err
		}
		blkID, err := c.db.Get(PrefixBlockHeightIDKey(expiryHeight))
		if err != nil {
			return fmt.Errorf("unable to fetch blockID at height %d: %w", expiryHeight, err)
		}
		if err := batch.Delete(PrefixBlockIDHeightKey(ids.ID(blkID))); err != nil {
			return err
		}
		if err := batch.Delete(PrefixBlockHeightIDKey(expiryHeight)); err != nil {
			return err
		}
		expired = true
		c.metrics.deletedBlocks.Inc()
	}
	if expired && rand.Intn(c.config.BlockCompactionAverageFrequency) == 0 {
		go func() {
			start := time.Now()
			if err := c.db.Compact([]byte{blockPrefix}, PrefixBlockKey(expiryHeight)); err != nil {
				c.log.Error("failed to compact block store", zap.Error(err))
				return
			}
			c.log.Info("compacted disk blocks", zap.Uint64("end", expiryHeight), zap.Duration("t", time.Since(start)))
		}()
	}

	return batch.Write()
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

func (c *ChainStore) GetResultByHeight(blkHeight uint64) ([]byte, error) {
	return c.db.Get(PrefixResultKey(blkHeight))
}

func PrefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func PrefixResultKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = resultPrefix
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
