// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

// TODO: gossip chunks as soon as build block (before verify)
// TODO: automatically fetch new chunks before needed (will better control
// which we fetch in the future)
type NodeChunks struct {
	Min         uint64
	Max         uint64
	Unprocessed []ids.ID
}

type ChunkHandler struct {
	vm *VM

	// TODO: track which heights held by peers
	// <min,max,[]unprocessed>
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

func (*ChunkHandler) AppGossip(context.Context, ids.NodeID, []byte) error {
	return nil
}

func (w *ChunkHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	return w.vm.warpManager.AppRequest(ctx, nodeID, requestID, request)
}

func (w *ChunkHandler) AppRequestFailed(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
) error {
	return w.vm.warpManager.HandleRequestFailed(requestID)
}

func (w *ChunkHandler) AppResponse(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	// TODO: return chunk if have it
	// Return min,max,unprocessed
	return w.vm.warpManager.HandleResponse(requestID, response)
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
