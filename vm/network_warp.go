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

func (w *WarpHandler) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	v *version.Application,
) error {
	return nil
}

func (w *WarpHandler) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return nil
}

func (w *WarpHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
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
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	return w.vm.warpManager.HandleRequestFailed(requestID)
}

func (w *WarpHandler) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return w.vm.warpManager.HandleResponse(requestID, response)
}

func (w *WarpHandler) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (w *WarpHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (w *WarpHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
