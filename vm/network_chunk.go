// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

type ChunkHandler struct {
	vm *VM
}

func NewChunkHandler(vm *VM) *ChunkHandler {
	return &ChunkHandler{vm}
}

func (*ChunkHandler) Connected(context.Context, ids.NodeID, *version.Application) error {
	return nil
}

func (*ChunkHandler) Disconnected(context.Context, ids.NodeID) error {
	return nil
}

func (c *ChunkHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return c.vm.chunkManager.HandleAppGossip(ctx, nodeID, msg)
}

func (c *ChunkHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	return c.vm.chunkManager.HandleRequest(ctx, nodeID, requestID, request)
}

func (c *ChunkHandler) AppRequestFailed(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
) error {
	return c.vm.chunkManager.HandleRequestFailed(requestID)
}

func (c *ChunkHandler) AppResponse(
	_ context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return c.vm.chunkManager.HandleResponse(nodeID, requestID, response)
}

func (*ChunkHandler) CrossChainAppRequest(context.Context, ids.ID, uint32, time.Time, []byte) error {
	return nil
}

func (*ChunkHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*ChunkHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
