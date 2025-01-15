// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
)

var _ block.StateSummary = (*SyncableBlock[StateSummaryBlock])(nil)

type StateSummaryBlock interface {
	fmt.Stringer
	GetID() ids.ID
	GetHeight() uint64
	GetBytes() []byte
	GetStateRoot() ids.ID
}

type SyncableBlock[T StateSummaryBlock] struct {
	container T
	accepter  *Client[T]
}

func NewSyncableBlock[T StateSummaryBlock](container T, accepter *Client[T]) *SyncableBlock[T] {
	return &SyncableBlock[T]{
		container: container,
		accepter:  accepter,
	}
}

func (sb *SyncableBlock[T]) ID() ids.ID {
	return sb.container.GetID()
}

func (sb *SyncableBlock[T]) Height() uint64 {
	return sb.container.GetHeight()
}

func (sb *SyncableBlock[T]) Bytes() []byte {
	return sb.container.GetBytes()
}

func (sb *SyncableBlock[T]) Accept(ctx context.Context) (block.StateSyncMode, error) {
	return sb.accepter.Accept(ctx, sb.container)
}

func (sb *SyncableBlock[T]) String() string {
	return sb.container.String()
}
