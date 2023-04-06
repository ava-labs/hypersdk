// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
	"go.uber.org/zap"
)

type ChunkHandler struct {
	vm *VM

	m  map[ids.NodeID]*NodeChunks
	ml sync.Mutex

	chunks     map[ids.ID][]byte
	chunksLock sync.Mutex
}

func NewChunkHandler(vm *VM) *ChunkHandler {
	return &ChunkHandler{
		vm:     vm,
		m:      map[ids.NodeID]*NodeChunks{},
		chunks: map[ids.ID][]byte{},
	}
}

func (*ChunkHandler) Connected(context.Context, ids.NodeID, *version.Application) error {
	return nil
}

func (*ChunkHandler) Disconnected(context.Context, ids.NodeID) error {
	return nil
}

func (c *ChunkHandler) AppGossip(_ context.Context, nodeID ids.NodeID, b []byte) error {
	nc, err := UnmarshalNodeChunks(b)
	if err != nil {
		c.vm.Logger().Error("unable to parse chunk gossip", zap.Error(err))
		return nil
	}
	c.ml.Lock()
	c.m[nodeID] = nc
	c.ml.Unlock()
	return nil
}

func (c *ChunkHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	chunkID, err := ids.ToID(request)
	if err != nil {
		c.vm.Logger().Error("unable to parse chunk request", zap.Error(err))
		return nil
	}
	// TODO: if don't have it, send back empty bytes, so can ask someone else
	return nil
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
