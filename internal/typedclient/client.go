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

type Marshaler[T any, U any, V any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
	MarshalGossip(V) ([]byte, error)
}

type TypedClient[T any, U any, V any] struct {
	client    *p2p.Client
	marshaler Marshaler[T, U, V]
}

func NewTypedClient[T any, U any, V any](client *p2p.Client, marshaler Marshaler[T, U, V]) *TypedClient[T, U, V] {
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
			// TODO how do we handle this?
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
