// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type ExecutionBlock[Container emap.Item] interface {
	GetParent() ids.ID
	GetTimestamp() int64
	GetHeight() uint64
	Containers() []Container
	Contains(ids.ID) bool
}

type ChainIndex[Container emap.Item] interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[Container], error)
}
