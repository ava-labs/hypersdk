// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

var _ BlockContext = &BlockCtx{}

type BlockCtx struct {
	height    uint64
	timestamp int64
}

func NewBlockContext(height uint64, timestamp int64) *BlockCtx {
	return &BlockCtx{
		height:    height,
		timestamp: timestamp,
	}
}

func (b *BlockCtx) Height() uint64 {
	return b.height
}

func (b *BlockCtx) Timestamp() int64 {
	return b.timestamp
}
