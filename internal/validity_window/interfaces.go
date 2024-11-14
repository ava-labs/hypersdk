// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validity_window

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/internal/emap"
)

type ExecutionBlock[TxnTypePtr emap.Item] interface {
	Parent() ids.ID
	Timestamp() int64
	Height() uint64
	Txs() []TxnTypePtr
	InitTxs() error
	ContainsTx(ids.ID) bool
}

type ChainIndex[TxnTypePtr emap.Item] interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[TxnTypePtr], error)
	LastAcceptedBlockHeight() uint64
}

type TimeValidityWindow[TxnTypePtr emap.Item] interface {
	Accept(blk ExecutionBlock[TxnTypePtr])
	IsRepeat(
		ctx context.Context,
		parentBlk ExecutionBlock[TxnTypePtr],
		txs []TxnTypePtr,
		oldestAllowed int64,
	) (set.Bits, error)
	VerifyExpiryReplayProtection(
		ctx context.Context,
		blk ExecutionBlock[TxnTypePtr],
		oldestAllowed int64,
	) error
}

type Syncer[TxnTypePtr emap.Item] interface {
	Accept(ctx context.Context, blk ExecutionBlock[TxnTypePtr]) (bool, error)
}
