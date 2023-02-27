// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

type StateSyncHandler struct {
	vm *VM
}

func NewStateSyncHandler(vm *VM) *StateSyncHandler {
	return &StateSyncHandler{vm}
}

func (s *StateSyncHandler) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	v *version.Application,
) error {
	return s.vm.stateSyncNetworkClient.Connected(ctx, nodeID, v)
}

func (s *StateSyncHandler) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return s.vm.stateSyncNetworkClient.Disconnected(ctx, nodeID)
}

func (*StateSyncHandler) AppGossip(context.Context, ids.NodeID, []byte) error {
	return nil
}

func (s *StateSyncHandler) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	if delay := s.vm.config.GetStateSyncServerDelay(); delay > 0 {
		time.Sleep(delay)
	}
	return s.vm.stateSyncNetworkServer.AppRequest(ctx, nodeID, requestID, deadline, request)
}

func (s *StateSyncHandler) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	return s.vm.stateSyncNetworkClient.AppRequestFailed(ctx, nodeID, requestID)
}

func (s *StateSyncHandler) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return s.vm.stateSyncNetworkClient.AppResponse(ctx, nodeID, requestID, response)
}

func (*StateSyncHandler) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*StateSyncHandler) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*StateSyncHandler) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
