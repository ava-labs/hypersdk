package dsmr

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/set"
)

type Marshaler[T any, U any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
}

type TypedClient[T any, U any] struct {
	*p2p.Client
	marshaler Marshaler[T, U]
}

// TODO merge upstream into avalanchego
func NewTypedClient[T any, U any](client *p2p.Client, marshaler Marshaler[T, U]) *TypedClient[T, U] {
	return &TypedClient[T, U]{
		Client:    client,
		marshaler: marshaler,
	}
}

// AppRequest issues an arbitrary request to a node.
// [onResponse] is invoked upon an error or a response.
func (c *TypedClient[T, U]) AppRequest(
	ctx context.Context,
	nodeIDs set.Set[ids.NodeID],
	request T,
	onResponse func(
		ctx context.Context,
		nodeID ids.NodeID,
		response U,
		err error,
	)) error {
	onByteResponse := func(ctx context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
		response, parseErr := c.marshaler.UnmarshalResponse(responseBytes)
		if parseErr != nil {
			//TODO how do we handle this?
			return
		}

		onResponse(ctx, nodeID, response, err)

	}

	requestBytes, err := c.marshaler.MarshalRequest(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return c.Client.AppRequest(
		ctx,
		nodeIDs,
		requestBytes,
		onByteResponse,
	)
}
