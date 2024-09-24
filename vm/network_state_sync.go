// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/chain"
)

type StateSyncHandler[T chain.PendingView] struct {
	vm *VM[T]
}

func NewStateSyncHandler[T chain.PendingView](vm *VM[T]) *StateSyncHandler[T] {
	return &StateSyncHandler[T]{vm}
}

func (s *StateSyncHandler[_]) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	v *version.Application,
) error {
	return s.vm.stateSyncNetworkClient.Connected(ctx, nodeID, v)
}

func (s *StateSyncHandler[_]) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return s.vm.stateSyncNetworkClient.Disconnected(ctx, nodeID)
}

func (*StateSyncHandler[_]) AppGossip(context.Context, ids.NodeID, []byte) error {
	return nil
}

func (s *StateSyncHandler[_]) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	if delay := s.vm.config.StateSyncServerDelay; delay > 0 {
		time.Sleep(delay)
	}
	return s.vm.stateSyncNetworkServer.AppRequest(ctx, nodeID, requestID, deadline, request)
}

func (s *StateSyncHandler[_]) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	return s.vm.stateSyncNetworkClient.AppRequestFailed(ctx, nodeID, requestID)
}

func (s *StateSyncHandler[_]) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	return s.vm.stateSyncNetworkClient.AppResponse(ctx, nodeID, requestID, response)
}

func (*StateSyncHandler[_]) CrossChainAppRequest(
	context.Context,
	ids.ID,
	uint32,
	time.Time,
	[]byte,
) error {
	return nil
}

func (*StateSyncHandler[_]) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*StateSyncHandler[_]) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}
