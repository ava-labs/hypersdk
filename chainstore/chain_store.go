// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainstore

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"math/rand"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/consts"
	hcontext "github.com/ava-labs/hypersdk/context"
	"go.uber.org/zap"
)

const namespace = "chainstore"

const (
	blockPrefix         byte = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix byte = 0x1 // ID -> Height
	blockHeightIDPrefix byte = 0x2 // Height -> ID (don't always need full block from disk)
	lastAcceptedByte    byte = 0x3 // lastAcceptedByte -> lastAcceptedHeight
)

type Config struct {
	AcceptedBlockWindow             uint64 `json:"acceptedBlockWindow"`
	BlockCompactionAverageFrequency int    `json:"blockCompactionFrequency"`
}

func NewDefaultConfig() Config {
	return Config{
		AcceptedBlockWindow:             50_000, // ~3.5hr with 250ms block time (100GB at 2MB)
		BlockCompactionAverageFrequency: 32,     // 64 MB of deletion if 2 MB blocks
	}
}

// ChainStore provides a persistent store that maps:
// blockHeight -> blockBytes
// blockID -> blockHeight
// blockHeight -> blockID
// TODO: add metrics / span tracing
type ChainStore[T Block] struct {
	config  Config
	metrics *metrics
	log     logging.Logger
	db      database.Database
	parser  Parser[T]
}

type Block interface {
	ID() ids.ID
	Height() uint64
	Bytes() []byte
}

type Parser[T Block] interface {
	ParseBlock(context.Context, []byte) (T, error)
}

func New[T Block](
	hctx *hcontext.Context,
	parser Parser[T],
	db database.Database,
) (*ChainStore[T], error) {
	registry, err := hctx.MakeRegistry(namespace)
	if err != nil {
		return nil, err
	}
	metrics, err := newMetrics(registry)
	if err != nil {
		return nil, err
	}
	config, err := hcontext.GetConfigFromContext(hctx, namespace, NewDefaultConfig())
	if err != nil {
		return nil, err
	}

	return &ChainStore[T]{
		config:  config,
		metrics: metrics,
		log:     hctx.Log(),
		db:      db,
		parser:  parser,
	}, nil
}

func (c *ChainStore[T]) GetLastAcceptedHeight(_ context.Context) (uint64, error) {
	lastAcceptedHeightBytes, err := c.db.Get([]byte{lastAcceptedByte})
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(lastAcceptedHeightBytes)
}

func (c *ChainStore[T]) Accept(_ context.Context, blk T) error {
	batch := c.db.NewBatch()

	var (
		blkID    = blk.ID()
		height   = blk.Height()
		blkBytes = blk.Bytes()
	)
	heightBytes := binary.BigEndian.AppendUint64(nil, height)
	err := errors.Join(
		batch.Put([]byte{lastAcceptedByte}, heightBytes),
		batch.Put(PrefixBlockIDHeightKey(blkID), heightBytes),
		batch.Put(PrefixBlockHeightIDKey(blk.Height()), blkID[:]),
		batch.Put(PrefixBlockKey(height), blkBytes),
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

func (c *ChainStore[T]) GetBlock(ctx context.Context, blkID ids.ID) (T, error) {
	var emptyT T
	height, err := c.GetBlockIDHeight(ctx, blkID)
	if err != nil {
		return emptyT, err
	}
	return c.GetBlockByHeight(ctx, height)
}

func (c *ChainStore[T]) GetBlockIDAtHeight(_ context.Context, blkHeight uint64) (ids.ID, error) {
	blkIDBytes, err := c.db.Get(PrefixBlockHeightIDKey(blkHeight))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(blkIDBytes), nil
}

func (c *ChainStore[T]) GetBlockIDHeight(_ context.Context, blkID ids.ID) (uint64, error) {
	blkHeightBytes, err := c.db.Get(PrefixBlockIDHeightKey(blkID))
	if err != nil {
		return 0, err
	}
	return database.ParseUInt64(blkHeightBytes)
}

func (c *ChainStore[T]) GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error) {
	var emptyT T
	blkBytes, err := c.db.Get(PrefixBlockKey(blkHeight))
	if err != nil {
		return emptyT, err
	}
	return c.parser.ParseBlock(ctx, blkBytes)
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
