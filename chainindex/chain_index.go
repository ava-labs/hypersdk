// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/consts"
)

const (
	blockPrefix         byte = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix byte = 0x1 // ID -> Height
	blockHeightIDPrefix byte = 0x2 // Height -> ID (don't always need full block from disk)
	lastAcceptedByte    byte = 0x3 // lastAcceptedByte -> lastAcceptedHeight
)

var (
	lastAcceptedKey = []byte{lastAcceptedByte}

	errBlockCompactionFrequencyZero = errors.New("block compaction frequency must be non-zero")
)

type Config struct {
	AcceptedBlockWindow      uint64 `json:"acceptedBlockWindow"`
	BlockCompactionFrequency uint64 `json:"blockCompactionFrequency"`
}

func NewDefaultConfig() Config {
	return Config{
		AcceptedBlockWindow:      50_000, // ~3.5hr with 250ms block time (100GB at 2MB)
		BlockCompactionFrequency: 32,     // 64 MB of deletion if 2 MB blocks
	}
}

type ChainIndex[T Block] struct {
	config           Config
	compactionOffset uint64
	metrics          *metrics
	log              logging.Logger
	db               database.Database
	parser           Parser[T]
}

type Block interface {
	GetID() ids.ID
	GetHeight() uint64
	GetBytes() []byte
}

type Parser[T Block] interface {
	ParseBlock(context.Context, []byte) (T, error)
}

func New[T Block](
	log logging.Logger,
	registry prometheus.Registerer,
	config Config,
	parser Parser[T],
	db database.Database,
) (*ChainIndex[T], error) {
	metrics, err := newMetrics(registry)
	if err != nil {
		return nil, err
	}
	if config.BlockCompactionFrequency == 0 {
		return nil, errBlockCompactionFrequencyZero
	}

	return &ChainIndex[T]{
		config: config,
		// Offset by random number to ensure the network does not compact simultaneously
		compactionOffset: rand.Uint64() % config.BlockCompactionFrequency, //nolint:gosec
		metrics:          metrics,
		log:              log,
		db:               db,
		parser:           parser,
	}, nil
}

func (c *ChainIndex[T]) GetLastAcceptedHeight(_ context.Context) (uint64, error) {
	lastAcceptedHeightBytes, err := c.db.Get(lastAcceptedKey)
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(lastAcceptedHeightBytes)
}

func (c *ChainIndex[T]) UpdateLastAccepted(ctx context.Context, blk T) error {
	batch := c.db.NewBatch()

	var (
		blkID    = blk.GetID()
		height   = blk.GetHeight()
		blkBytes = blk.GetBytes()
	)
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	err := errors.Join(
		batch.Put(lastAcceptedKey, heightBytes),
		batch.Put(prefixBlockIDHeightKey(blkID), heightBytes),
		batch.Put(prefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(prefixBlockKey(height), blkBytes),
	)
	if err != nil {
		return err
	}

	expiryHeight := height - c.config.AcceptedBlockWindow
	if c.config.AcceptedBlockWindow == 0 || expiryHeight == 0 || expiryHeight >= height { // ensure we don't free genesis
		return batch.Write()
	}

	if err := batch.Delete(prefixBlockKey(expiryHeight)); err != nil {
		return err
	}
	deleteBlkID, err := c.GetBlockIDAtHeight(ctx, expiryHeight)
	if err != nil {
		return err
	}
	if err := batch.Delete(prefixBlockIDHeightKey(deleteBlkID)); err != nil {
		return err
	}
	if err := batch.Delete(prefixBlockHeightIDKey(expiryHeight)); err != nil {
		return err
	}
	c.metrics.deletedBlocks.Inc()

	if expiryHeight%c.config.BlockCompactionFrequency == c.compactionOffset {
		go func() {
			start := time.Now()
			if err := c.db.Compact([]byte{blockPrefix}, prefixBlockKey(expiryHeight)); err != nil {
				c.log.Error("failed to compact block store", zap.Error(err))
				return
			}
			c.log.Info("compacted disk blocks", zap.Uint64("end", expiryHeight), zap.Duration("t", time.Since(start)))
		}()
	}

	return batch.Write()
}

func (c *ChainIndex[T]) GetBlock(ctx context.Context, blkID ids.ID) (T, error) {
	height, err := c.GetBlockIDHeight(ctx, blkID)
	if err != nil {
		return utils.Zero[T](), err
	}
	return c.GetBlockByHeight(ctx, height)
}

func (c *ChainIndex[T]) GetBlockIDAtHeight(_ context.Context, blkHeight uint64) (ids.ID, error) {
	blkIDBytes, err := c.db.Get(prefixBlockHeightIDKey(blkHeight))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(blkIDBytes), nil
}

func (c *ChainIndex[T]) GetBlockIDHeight(_ context.Context, blkID ids.ID) (uint64, error) {
	blkHeightBytes, err := c.db.Get(prefixBlockIDHeightKey(blkID))
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(blkHeightBytes)
}

func (c *ChainIndex[T]) GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error) {
	blkBytes, err := c.db.Get(prefixBlockKey(blkHeight))
	if err != nil {
		return utils.Zero[T](), err
	}
	return c.parser.ParseBlock(ctx, blkBytes)
}

func prefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func prefixBlockIDHeightKey(id ids.ID) []byte {
	k := make([]byte, 1+ids.IDLen)
	k[0] = blockIDHeightPrefix
	copy(k[1:], id[:])
	return k
}

func prefixBlockHeightIDKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}
