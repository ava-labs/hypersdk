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
	ID() ids.ID
	Height() uint64
	Bytes() []byte
	GetStateRoot() ids.ID
	MarkAccepted(context.Context)
}

type SyncableBlock[T StateSummaryBlock] struct {
	container T
	accepter  Accepter[T] // accepter is nil if the SyncableBlock is constructed by the server
}

func NewSyncableBlock[T StateSummaryBlock](container T, accepter Accepter[T]) *SyncableBlock[T] {
	return &SyncableBlock[T]{
		container: container,
		accepter:  accepter,
	}
}

func (sb *SyncableBlock[T]) ID() ids.ID {
	return sb.container.ID()
}

func (sb *SyncableBlock[T]) Height() uint64 {
	return sb.container.Height()
}

func (sb *SyncableBlock[T]) Bytes() []byte {
	return sb.container.Bytes()
}

func (sb *SyncableBlock[T]) Accept(ctx context.Context) (block.StateSyncMode, error) {
	return sb.accepter.Accept(ctx, sb.container)
}

func (sb *SyncableBlock[T]) MarkAccepted(ctx context.Context) {
	sb.container.MarkAccepted(ctx)
}

func (sb *SyncableBlock[T]) String() string {
	return sb.container.String()
}
