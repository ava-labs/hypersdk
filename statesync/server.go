// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

type ChainServer[T StateSummaryBlock] interface {
	LastAcceptedBlock(ctx context.Context) T
	GetBlockByHeight(ctx context.Context, height uint64) (T, error)
}

type Server[T StateSummaryBlock] struct {
	chain ChainServer[T]
	log   logging.Logger
}

func NewServer[T StateSummaryBlock](log logging.Logger, chain ChainServer[T]) *Server[T] {
	return &Server[T]{
		chain: chain,
		log:   log,
	}
}

// GetLastStateSummary returns the latest state summary.
// If no summary is available, [database.ErrNotFound] must be returned.
func (s *Server[T]) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	summary := NewSyncableBlock(s.chain.LastAcceptedBlock(ctx), nil)
	s.log.Info("Serving syncable block at latest height", zap.Stringer("summary", summary))
	return summary, nil
}

// GetStateSummary implements StateSyncableVM and returns a summary corresponding
// to the provided [height] if the node can serve state sync data for that key.
// If not, [database.ErrNotFound] must be returned.
func (s *Server[T]) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
	block, err := s.chain.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	summary := NewSyncableBlock(block, nil)
	s.log.Info("Serving syncable block at requested height",
		zap.Uint64("height", height),
		zap.Stringer("summary", summary),
	)
	return summary, nil
}
