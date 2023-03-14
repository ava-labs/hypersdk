// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

type WarpHandler struct {
	vm *VM
}

func NewWarpHandler(vm *VM) *WarpHandler {
	return &WarpHandler{vm}
}

func (*WarpHandler) Connected(context.Context, ids.NodeID, *version.Application) error {
	return nil
}

func (*WarpHandler) Disconnected(context.Context, ids.NodeID) error {
	return nil
}

func (*WarpHandler) AppGossip(context.Context, ids.NodeID, []byte) error {
	return nil
}

func (w *WarpHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	return w.vm.warpManager.AppRequest(ctx, nodeID, requestID, request)
}

func (w *WarpHandler) AppRequestFailed(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
) error {
	return w.vm.warpManager.HandleRequestFailed(requestID)
}

func (w *WarpHandler) AppResponse(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return w.vm.warpManager.HandleResponse(requestID, response)
}

func (*WarpHandler) CrossChainAppRequest(context.Context, ids.ID, uint32, time.Time, []byte) error {
	return nil
}

func (*WarpHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*WarpHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
