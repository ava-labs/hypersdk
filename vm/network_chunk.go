// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// TODO: gossip chunks as soon as build block (before verify)
// TODO: automatically fetch new chunks before needed (will better control
// which we fetch in the future)
// TODO: allow for deleting block chunks after some period of time
type NodeChunks struct {
	Min         uint64
	Max         uint64
	Unprocessed []ids.ID
}

func (n *NodeChunks) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackUint64(n.Min)
	p.PackUint64(n.Max)
	l := len(n.Unprocessed)
	p.PackInt(l)
	if l > 0 {
		for _, chunk := range n.Unprocessed {
			p.PackID(chunk)
		}
	}
	return p.Bytes(), p.Err()
}

func UnmarshalNodeChunks(b []byte) (*NodeChunks, error) {
	var n NodeChunks
	p := codec.NewReader(b, consts.MaxInt)
	n.Min = p.UnpackUint64(false) // could be genesis
	n.Max = p.UnpackUint64(false) // could be genesis
	l := p.UnpackInt(false)       // could have no processing
	n.Unprocessed = make([]ids.ID, l)
	for i := 0; i < l; i++ {
		p.UnpackID(true, &n.Unprocessed[i])
	}
	return &n, p.Err()
}

type ChunkHandler struct {
	vm *VM

	m map[ids.NodeID]*NodeChunks
	// TODO: track which heights held by peers
	// <min,max,[]unprocessed>
}

func NewChunkHandler(vm *VM) *ChunkHandler {
	return &ChunkHandler{vm, map[ids.NodeID]*NodeChunks{}}
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
