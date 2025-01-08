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

func (v *vm[I, O, A]) AppRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, deadline time.Time, request []byte) error {
	return v.app.Network.AppRequest(ctx, nodeID, requestID, deadline, request)
}

func (v *vm[I, O, A]) AppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
	return v.app.Network.AppResponse(ctx, nodeID, requestID, response)
}

func (v *vm[I, O, A]) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32, appErr *common.AppError) error {
	return v.app.Network.AppRequestFailed(ctx, nodeID, requestID, appErr)
}

func (v *vm[I, O, A]) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return v.app.Network.AppGossip(ctx, nodeID, msg)
}

func (v *vm[I, O, A]) Connected(ctx context.Context, nodeID ids.NodeID, nodeVersion *version.Application) error {
	return v.app.Network.Connected(ctx, nodeID, nodeVersion)
}

func (v *vm[I, O, A]) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return v.app.Network.Disconnected(ctx, nodeID)
}
