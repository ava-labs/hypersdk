// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
)

type TxGossipHandler[T chain.PendingView] struct {
	vm *VM[T]
}

func NewTxGossipHandler[T chain.PendingView](vm *VM[T]) *TxGossipHandler[T] {
	return &TxGossipHandler[T]{vm}
}

func (*TxGossipHandler[_]) Connected(context.Context, ids.NodeID, *version.Application) error {
	return nil
}

func (*TxGossipHandler[_]) Disconnected(context.Context, ids.NodeID) error {
	return nil
}

func (t *TxGossipHandler[_]) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	if !t.vm.isReady() {
		t.vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(ErrNotReady))
		return nil
	}

	return t.vm.gossiper.HandleAppGossip(ctx, nodeID, msg)
}

func (*TxGossipHandler[_]) AppRequest(
	context.Context,
	ids.NodeID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler[_]) AppRequestFailed(
	context.Context,
	ids.NodeID,
	uint32,
) error {
	return nil
}

func (*TxGossipHandler[_]) AppResponse(
	context.Context,
	ids.NodeID,
	uint32,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler[_]) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler[_]) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*TxGossipHandler[_]) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
