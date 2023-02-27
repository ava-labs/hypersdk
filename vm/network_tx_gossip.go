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

func (t *TxGossipHandler) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	v *version.Application,
) error {
	return nil
}

func (t *TxGossipHandler) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return nil
}

func (t *TxGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	if !t.vm.isReady() {
		t.vm.snowCtx.Log.Warn("handle app gossip failed", zap.Error(ErrNotReady))
		return nil
	}

	return t.vm.gossiper.HandleAppGossip(ctx, nodeID, msg)
}

func (t *TxGossipHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	return nil
}

func (t *TxGossipHandler) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	return nil
}

func (t *TxGossipHandler) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return nil
}

func (t *TxGossipHandler) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (t *TxGossipHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (t *TxGossipHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
