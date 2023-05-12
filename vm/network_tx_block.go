// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

type TxBlockHandler struct {
	vm *VM
}

func NewTxBlockHandler(vm *VM) *TxBlockHandler {
	return &TxBlockHandler{vm}
}

func (c *TxBlockHandler) Connected(ctx context.Context, nodeID ids.NodeID, _ *version.Application) error {
	return c.vm.txBlockManager.HandleConnect(ctx, nodeID)
}

func (c *TxBlockHandler) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return c.vm.txBlockManager.HandleDisconnect(ctx, nodeID)
}

func (c *TxBlockHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return c.vm.txBlockManager.HandleAppGossip(ctx, nodeID, msg)
}

func (c *TxBlockHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	return c.vm.txBlockManager.HandleRequest(ctx, nodeID, requestID, request)
}

func (c *TxBlockHandler) AppRequestFailed(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
) error {
	return c.vm.txBlockManager.HandleRequestFailed(requestID)
}

func (c *TxBlockHandler) AppResponse(
	_ context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return c.vm.txBlockManager.HandleResponse(nodeID, requestID, response)
}

func (*TxBlockHandler) CrossChainAppRequest(context.Context, ids.ID, uint32, time.Time, []byte) error {
	return nil
}

func (*TxBlockHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*TxBlockHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
