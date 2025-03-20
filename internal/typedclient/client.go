// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package typedclient

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/set"
)

var _ P2PClient = (*p2p.Client)(nil)

// P2PClient defines an interface for peer-to-peer communication.
// This interface exists to:
// 1. Enable easier testing by allowing p2p.Client to be mocked
// 2. Provide a stable API boundary between our code and the upstream library
type P2PClient interface {
	AppRequestAny(ctx context.Context, appRequestBytes []byte, onResponse p2p.AppResponseCallback) error
	AppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], appRequestBytes []byte, onResponse p2p.AppResponseCallback) error
	AppGossip(ctx context.Context, config common.SendConfig, appGossipBytes []byte) error
}

type Marshaler[T any, U any, V any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
	MarshalGossip(V) ([]byte, error)
}

// TypedClient provides asynchronous type-safe wrapper around the P2PClient interface.
// Type parameters:
//   - T: Request type that gets marshaled and sent to peers
//   - U: Response type that gets unmarshaled from peer responses
//   - V: Gossip message type that gets marshaled and broadcast to peers
type TypedClient[T any, U any, V any] struct {
	client    P2PClient
	marshaler Marshaler[T, U, V]
}

func NewTypedClient[T any, U any, V any](client P2PClient, marshaler Marshaler[T, U, V]) *TypedClient[T, U, V] {
	return &TypedClient[T, U, V]{
		client:    client,
		marshaler: marshaler,
	}
}

func (t *TypedClient[T, U, _]) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	request T,
	onResponse func(ctx context.Context, nodeID ids.NodeID, response U, err error),
) error {
	onByteResponse := func(ctx context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		if err != nil {
			onResponse(ctx, nodeID, utils.Zero[U](), err)
			return
		}

		response, parseErr := t.marshaler.UnmarshalResponse(responseBytes)
		if parseErr != nil {
			onResponse(ctx, nodeID, utils.Zero[U](), parseErr)
			return
		}

		onResponse(ctx, nodeID, response, err)
	}

	requestBytes, err := t.marshaler.MarshalRequest(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return t.client.AppRequest(
		ctx,
		set.Of(nodeID),
		requestBytes,
		onByteResponse,
	)
}

func (t *TypedClient[T, U, V]) AppGossip(
	ctx context.Context,
	gossip V,
) error {
	gossipBytes, err := t.marshaler.MarshalGossip(gossip)
	if err != nil {
		return fmt.Errorf("failed to marshal gossip: %w", err)
	}

	return t.client.AppGossip(
		ctx,
		common.SendConfig{
			Validators: 100,
		},
		gossipBytes,
	)
}
