// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
)

var _ block.StateSyncableVM = (*VM[StateSummaryBlock])(nil)

type VM[T StateSummaryBlock] struct {
	client *Client[T]
	server *Server[T]
}

func NewStateSyncableVM[T StateSummaryBlock](
	client *Client[T],
	server *Server[T],
) *VM[T] {
	return &VM[T]{
		client: client,
		server: server,
	}
}

func (v *VM[T]) StateSyncEnabled(ctx context.Context) (bool, error) {
	return v.client.StateSyncEnabled(ctx)
}

func (v *VM[T]) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.client.GetOngoingSyncStateSummary(ctx)
}

func (v *VM[T]) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.server.GetLastStateSummary(ctx)
}

func (v *VM[T]) ParseStateSummary(ctx context.Context, summaryBytes []byte) (block.StateSummary, error) {
	return v.client.ParseStateSummary(ctx, summaryBytes)
}

func (v *VM[T]) GetStateSummary(ctx context.Context, summaryHeight uint64) (block.StateSummary, error) {
	return v.server.GetStateSummary(ctx, summaryHeight)
}
