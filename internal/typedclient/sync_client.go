// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package typedclient

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
)

// SyncTypedClient provides a synchronous wrapper around TypedClient
type SyncTypedClient[T any, U any, V any] struct {
	client *TypedClient[T, U, V]
}

// NewSyncTypedClient creates a new synchronous client
func NewSyncTypedClient[T any, U any, V any](client P2PClient, marshaler Marshaler[T, U, V]) *SyncTypedClient[T, U, V] {
	return &SyncTypedClient[T, U, V]{
		client: NewTypedClient(client, marshaler),
	}
}

// SyncAppRequest sends a request and waits for the response synchronously
func (s *SyncTypedClient[T, U, V]) SyncAppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	request T,
) (U, error) {
	var (
		response U
		respErr  error
		done     = make(chan struct{})
	)

	// We are guaranteed to eventually receive a response
	onResponse := func(_ context.Context, _ ids.NodeID, resp U, err error) {
		select {
		case <-done:
			return
		default:
			response = resp
			respErr = err
			close(done)
		}
	}

	if err := s.client.AppRequest(ctx, nodeID, request, onResponse); err != nil {
		return utils.Zero[U](), err
	}

	select {
	case <-done:
		return response, respErr
	case <-ctx.Done():
		return utils.Zero[U](), ctx.Err()
	}
}
