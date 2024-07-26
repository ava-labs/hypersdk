// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
	"go.uber.org/zap"
)

type TxGossipHandler struct {
	vm *VM
}

func NewTxGossipHandler(vm *VM) *TxGossipHandler {
	return &TxGossipHandler{vm}
}

func (*TxGossipHandler) Connected(context.Context, ids.NodeID, *version.Application) error {
	return nil
}

func (*TxGossipHandler) Disconnected(context.Context, ids.NodeID) error {
	return nil
}

func (t *TxGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	if !t.vm.isReady() {
		t.vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(ErrNotReady))
		return nil
	}

	return t.vm.gossiper.HandleAppGossip(ctx, nodeID, msg)
}

func (*TxGossipHandler) AppRequest(
	context.Context,
	ids.NodeID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler) AppRequestFailed(
	context.Context,
	ids.NodeID,
	uint32,
) error {
	return nil
}

func (*TxGossipHandler) AppResponse(
	context.Context,
	ids.NodeID,
	uint32,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*TxGossipHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*TxGossipHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
