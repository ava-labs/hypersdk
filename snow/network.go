// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/version"
)

var _ common.AppHandler = (*VM[Block, Block, Block])(nil)

// AppRequest sends async request to a node
func (v *VM[I, O, A]) AppRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, deadline time.Time, request []byte) error {
	return v.network.AppRequest(ctx, nodeID, requestID, deadline, request)
}

// AppResponse receives async response from a node
func (v *VM[I, O, A]) AppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
	return v.network.AppResponse(ctx, nodeID, requestID, response)
}

// AppRequestFailed checks if a request failed and returns the error if it did
func (v *VM[I, O, A]) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32, appErr *common.AppError) error {
	return v.network.AppRequestFailed(ctx, nodeID, requestID, appErr)
}

// AppGossip sends gossip message to a node
func (v *VM[I, O, A]) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return v.network.AppGossip(ctx, nodeID, msg)
}

// Connected is called when a node is connected to the network
func (v *VM[I, O, A]) Connected(ctx context.Context, nodeID ids.NodeID, nodeVersion *version.Application) error {
	return v.network.Connected(ctx, nodeID, nodeVersion)
}

// Disconnected is called when a node is disconnected from the network
func (v *VM[I, O, A]) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return v.network.Disconnected(ctx, nodeID)
}
