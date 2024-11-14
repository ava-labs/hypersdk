// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type ExecutionBlock[Container emap.Item] interface {
	Parent() ids.ID
	Timestamp() int64
	Height() uint64
	Txs() []Container
	InitTxs() error
	ContainsTx(ids.ID) bool
}

type ChainIndex[Container emap.Item] interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[Container], error)
	LastAcceptedBlockHeight() uint64
}

type TimeValidityWindow[Container emap.Item] interface {
	Accept(blk ExecutionBlock[Container])
	IsRepeat(
		ctx context.Context,
		parentBlk ExecutionBlock[Container],
		txs []Container,
		oldestAllowed int64,
	) (set.Bits, error)
	VerifyExpiryReplayProtection(
		ctx context.Context,
		blk ExecutionBlock[Container],
		oldestAllowed int64,
	) error
}

type Syncer[Container emap.Item] interface {
	Accept(ctx context.Context, blk ExecutionBlock[Container]) (bool, error)
}
