// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindowtest

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

type MockChainIndex[T emap.Item] struct {
	blocks map[ids.ID]validitywindow.ExecutionBlock[T]
}

func (m *MockChainIndex[T]) GetExecutionBlock(_ context.Context, blkID ids.ID) (validitywindow.ExecutionBlock[T], error) {
	if blk, ok := m.blocks[blkID]; ok {
		return blk, nil
	}
	return nil, database.ErrNotFound
}

func (m *MockChainIndex[T]) Set(blkID ids.ID, blk validitywindow.ExecutionBlock[T]) {
	if m.blocks == nil {
		m.blocks = make(map[ids.ID]validitywindow.ExecutionBlock[T])
	}
	m.blocks[blkID] = blk
}
