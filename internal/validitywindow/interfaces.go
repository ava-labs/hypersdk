// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type ExecutionBlock[Container emap.Item] interface {
	Parent() ids.ID
	Timestamp() int64
	Height() uint64
	Txs() []Container
	ContainsTx(ids.ID) bool
}

type ChainIndex[Container emap.Item] interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[Container], error)
}
