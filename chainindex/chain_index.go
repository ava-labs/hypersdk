// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

const (
	blockPrefix         byte = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockHeightIDPrefix byte = 0x1 // Height -> ID (don't always need full block from disk)
	lastAcceptedByte    byte = 0x2 // lastAcceptedByte -> lastAcceptedHeight
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
	db               pebble.ExtendedDatabase
	parser           Parser[T]
	idToHeight       map[ids.ID]uint64
	idToHeightLock   sync.RWMutex
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

	extendedDB, err := pebble.AsExtendedDatabase(db)
	if err != nil {
		return nil, err
	}

	ci := &ChainIndex[T]{
		config: config,
		// Offset by random number to ensure the network does not compact simultaneously
		compactionOffset: rand.Uint64() % config.BlockCompactionFrequency, //nolint:gosec
		metrics:          metrics,
		log:              log,
		db:               extendedDB,
		parser:           parser,
	}

	if err := ci.rebuildBlkIDMapping(); err != nil {
		return nil, fmt.Errorf("failed to rebuild ID mapping: %w", err)
	}

	return ci, nil
}

func (c *ChainIndex[T]) GetLastAcceptedHeight(_ context.Context) (uint64, error) {
	lastAcceptedHeightBytes, err := c.db.Get(lastAcceptedKey)
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(lastAcceptedHeightBytes)
}

func (c *ChainIndex[T]) UpdateLastAccepted(_ context.Context, blk T) error {
	batch := c.db.NewBatch()

	var (
		blkID    = blk.GetID()
		height   = blk.GetHeight()
		blkBytes = blk.GetBytes()
	)
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	err := errors.Join(
		batch.Put(lastAcceptedKey, heightBytes),
		batch.Put(prefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(prefixBlockKey(height), blkBytes),
	)
	if err != nil {
		return err
	}

	if err := c.write(batch, blkID, height); err != nil {
		return err
	}

	expiryHeight := height - c.config.AcceptedBlockWindow
	if c.config.AcceptedBlockWindow == 0 || expiryHeight == 0 || expiryHeight >= height {
		return nil
	}

	c.idToHeightLock.RLock()
	pruningStartHeight, removeIDs := c.pruningRange(height)
	c.idToHeightLock.RUnlock()

	if delErr := c.deleteRange(pruningStartHeight, expiryHeight); delErr != nil {
		return delErr
	}

	c.idToHeightLock.Lock()
	for _, id := range removeIDs {
		delete(c.idToHeight, id)
		c.metrics.deletedBlocks.Inc()
	}
	c.idToHeightLock.Unlock()

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

	return nil
}

// WriteBlock stores a block in the database without enforcing any pruning
// It is intended only for initial state sync.
// It relies on UpdateLastAccepted to handle cleanup based on the final accepted height
// DO NOT use this method during normal operation - use UpdateLastAccepted instead
func (c *ChainIndex[T]) WriteBlock(blk Block) error {
	batch := c.db.NewBatch()

	var (
		blkID    = blk.GetID()
		height   = blk.GetHeight()
		blkBytes = blk.GetBytes()
	)
	err := errors.Join(
		batch.Put(prefixBlockHeightIDKey(height), blkID[:]),
		batch.Put(prefixBlockKey(height), blkBytes),
	)
	if err != nil {
		return err
	}

	return c.write(batch, blkID, height)
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
	c.idToHeightLock.RLock()
	defer c.idToHeightLock.RUnlock()
	height, ok := c.idToHeight[blkID]

	if ok {
		return height, nil
	}
	return 0, database.ErrNotFound
}

func (c *ChainIndex[T]) GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error) {
	blkBytes, err := c.db.Get(prefixBlockKey(blkHeight))
	if err != nil {
		return utils.Zero[T](), err
	}
	return c.parser.ParseBlock(ctx, blkBytes)
}

func (c *ChainIndex[T]) rebuildBlkIDMapping() error {
	c.idToHeightLock.Lock()
	defer c.idToHeightLock.Unlock()

	c.idToHeight = make(map[ids.ID]uint64)
	it := c.db.NewIteratorWithPrefix([]byte{blockHeightIDPrefix})
	defer it.Release()

	for it.Next() {
		key := it.Key()
		value := it.Value()
		height := binary.BigEndian.Uint64(key[1:])
		var id ids.ID
		copy(id[:], value)
		c.idToHeight[id] = height
	}

	return it.Error()
}

func prefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

// prefixBlockKeyEnd creates an exclusive upper bound for range operations by incrementing
// the height. For example, to delete blocks 1-5, we need a range of [prefixBlockKey(1), prefixBlockKeyEnd(5)].
// Without this adjustment, the range [prefixBlockKey(1), prefixBlockKeyEnd(5)] would miss block 5 since range
// operations are end-exclusive
func prefixBlockKeyEnd(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height+1)
	return k
}

func prefixBlockHeightIDKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

// prefixBlockHeightIDKeyEnd follows the same pattern as prefixBlockKeyEnd
func prefixBlockHeightIDKeyEnd(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height+1)
	return k
}

func (c *ChainIndex[T]) write(batch database.Batch, blkID ids.ID, height uint64) error {
	err := batch.Write()
	if err != nil {
		return err
	}

	c.idToHeightLock.Lock()
	c.idToHeight[blkID] = height
	c.idToHeightLock.Unlock()
	return nil
}

func (c *ChainIndex[T]) deleteRange(startHeight, endHeight uint64) error {
	var (
		startKey         = prefixBlockKey(startHeight)
		endKey           = prefixBlockKeyEnd(endHeight)
		startHeightIDKey = prefixBlockHeightIDKey(startHeight)
		endHeightIDKey   = prefixBlockHeightIDKeyEnd(endHeight)
	)
	if err := c.db.DeleteRange(startKey, endKey); err != nil {
		return fmt.Errorf("failed to delete expired blocks in range [%d, %d): %w",
			startHeight, endHeight, err)
	}
	if err := c.db.DeleteRange(startHeightIDKey, endHeightIDKey); err != nil {
		return fmt.Errorf("failed to delete expired block height mappings in range [%d, %d): %w",
			startHeight, endHeight, err)
	}

	return nil
}

// pruningRange returns the range of associated block IDs that should be pruned. This is used to determine the
// starting point for our block deletion window
func (c *ChainIndex[T]) pruningRange(currentHeight uint64) (uint64, []ids.ID) {
	// Find minimum height (excluding genesis at 0) and collect IDs to remove
	minHeight := currentHeight
	pruneEndHeight := currentHeight - c.config.AcceptedBlockWindow
	removeIDs := make([]ids.ID, 0)

	for id, height := range c.idToHeight {
		// Skip genesis and current block
		if height == 0 || height == currentHeight {
			continue
		}

		if height < minHeight {
			minHeight = height
		}

		if height <= pruneEndHeight {
			removeIDs = append(removeIDs, id)
		}
	}

	return minHeight, removeIDs
}
