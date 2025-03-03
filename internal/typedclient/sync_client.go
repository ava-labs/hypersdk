// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package typedclient

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils"
)

// SyncTypedClient provides a synchronous interface for the TypedClient
type SyncTypedClient[T any, U any, V any] struct {
	client *TypedClient[T, U, V]
}

// NewSyncTypedClient creates a new synchronous client with the default timeout of 30 seconds
func NewSyncTypedClient[T any, U any, V any](client *p2p.Client, marshaler Marshaler[T, U, V]) *SyncTypedClient[T, U, V] {
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
	type responseWrapper struct {
		resp U
		err  error
	}
	respCh := make(chan responseWrapper, 1)

	// We are guaranteed to eventually receive response
	onResponse := func(_ context.Context, _ ids.NodeID, resp U, respErr error) {
		select {
		case respCh <- responseWrapper{resp: resp, err: respErr}:
		default:
			// Channel is either full or closed - can happen if timeout already occurred
		}
	}

	if err := s.client.AppRequest(ctx, nodeID, request, onResponse); err != nil {
		return utils.Zero[U](), err
	}

	// Wait for either a response or context cancellation
	select {
	case wrapper := <-respCh:
		return wrapper.resp, wrapper.err
	case <-ctx.Done():
		// Context was canceled or timed out
		return utils.Zero[U](), ctx.Err()
	}
}
