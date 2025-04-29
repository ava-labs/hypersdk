// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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
	ctx context.Context,
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

	ci := &ChainIndex[T]{
		config: config,
		// Offset by random number to ensure the network does not compact simultaneously
		compactionOffset: rand.Uint64() % config.BlockCompactionFrequency, //nolint:gosec
		metrics:          metrics,
		log:              log,
		db:               db,
		parser:           parser,
	}

	return ci, ci.cleanupOnStartup(ctx)
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

	height := blk.GetHeight()
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	if err := errors.Join(
		batch.Put(lastAcceptedKey, heightBytes),
		c.writeBlock(batch, blk)); err != nil {
		return err
	}

	expiryHeight := height - c.config.AcceptedBlockWindow
	if c.config.AcceptedBlockWindow == 0 || expiryHeight == 0 || expiryHeight >= height { // ensure we don't free genesis
		return batch.Write()
	}

	deleteBlkID, err := c.GetBlockIDAtHeight(ctx, expiryHeight)
	if err != nil {
		return err
	}
	if err = errors.Join(
		batch.Delete(prefixBlockKey(expiryHeight)),
		batch.Delete(prefixBlockIDHeightKey(deleteBlkID)),
		batch.Delete(prefixBlockHeightIDKey(expiryHeight)),
	); err != nil {
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

// SaveHistorical writes block on-disk, without updating lastAcceptedKey,
// It should be used only for historical blocks, it's relying on heuristic of eventually calling UpdateLastAccepted,
// which will delete expired blocks
func (c *ChainIndex[T]) SaveHistorical(blk T) error {
	batch := c.db.NewBatch()
	if err := c.writeBlock(batch, blk); err != nil {
		return err
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

func (_ *ChainIndex[T]) writeBlock(batch database.Batch, blk T) error {
	var (
		blkID    = blk.GetID()
		height   = blk.GetHeight()
		blkBytes = blk.GetBytes()
	)
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	return errors.Join(
		batch.Put(prefixBlockIDHeightKey(blkID), heightBytes),
		batch.Put(prefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(prefixBlockKey(height), blkBytes),
	)
}

// cleanupOnStartup performs cleanup of historical blocks outside the accepted window.
//
// The cleanup removes all blocks below this threshold:
// | <--- Historical Blocks (delete) ---> | <--- AcceptedBlockWindow ---> | Last Accepted |
func (c *ChainIndex[T]) cleanupOnStartup(ctx context.Context) error {
	lastAcceptedHeight, err := c.GetLastAcceptedHeight(ctx)
	if err != nil && err != database.ErrNotFound {
		return err
	}

	// If there's no accepted window or lastAcceptedHeight is too small, nothing to clean
	if c.config.AcceptedBlockWindow == 0 || lastAcceptedHeight <= c.config.AcceptedBlockWindow {
		return nil
	}

	thresholdHeight := lastAcceptedHeight - c.config.AcceptedBlockWindow

	c.log.Debug("cleaning up historical blocks outside accepted window",
		zap.Uint64("lastAcceptedHeight", lastAcceptedHeight),
		zap.Uint64("thresholdHeight", thresholdHeight),
		zap.Uint64("acceptedBlockWindow", c.config.AcceptedBlockWindow))

	it := c.db.NewIteratorWithPrefix([]byte{blockHeightIDPrefix})
	defer it.Release()

	batch := c.db.NewBatch()
	var lastDeletedHeight uint64

	for it.Next() {
		key := it.Key()
		height := extractBlockHeightFromKey(key)

		// Nothing to delete after the threshold height
		if height >= thresholdHeight {
			break
		}

		// Skip if:
		// Block is at genesis height (0)
		if height == 0 {
			continue
		}

		deleteBlkID, err := c.GetBlockIDAtHeight(ctx, height)
		if err != nil {
			return err
		}

		if err = errors.Join(
			batch.Delete(prefixBlockKey(height)),
			batch.Delete(prefixBlockIDHeightKey(deleteBlkID)),
			batch.Delete(prefixBlockHeightIDKey(height)),
		); err != nil {
			return err
		}
		c.metrics.deletedBlocks.Inc()

		// Keep track of the last height we deleted
		lastDeletedHeight = height
	}

	if err := it.Error(); err != nil {
		return fmt.Errorf("iterator error during cleanup: %w", err)
	}

	// Write all the deletions
	if err := batch.Write(); err != nil {
		return err
	}

	// Perform a single compaction at the end if we deleted anything
	if lastDeletedHeight > 0 {
		go func() {
			start := time.Now()
			if err := c.db.Compact([]byte{blockPrefix}, prefixBlockKey(lastDeletedHeight)); err != nil {
				c.log.Error("failed to compact block store", zap.Error(err))
				return
			}
			c.log.Info("compacted disk blocks", zap.Uint64("end", lastDeletedHeight), zap.Duration("t", time.Since(start)))
		}()
	}

	return nil
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

// extractBlockHeightFromKey extracts block height from the key.
// The key is expected to be in the format: [1-byte prefix][8-byte big-endian encoded uint64]
func extractBlockHeightFromKey(key []byte) uint64 {
	return binary.BigEndian.Uint64(key[1:])
}
