// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"go.uber.org/zap"
)

var _ p2p.Handler = (*TxGossipHandler)(nil)

type TxGossipHandler struct {
	p2p.NoOpHandler
	vm *VM
}

func NewTxGossipHandler(vm *VM) *TxGossipHandler {
	return &TxGossipHandler{vm: vm}
}

func (t *TxGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) {
	if !t.vm.isReady() {
		t.vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(ErrNotReady))
		return
	}

	if err := t.vm.gossiper.HandleAppGossip(ctx, nodeID, msg); err != nil {
		t.vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(err))
	}
}
