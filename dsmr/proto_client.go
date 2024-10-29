package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/set"
)

// TODO merge into avalanchego
type Marshaler[T any, U any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
}

type TypedClient[T any, U any] struct {
	*p2p.Client
	marshaler Marshaler[T, U]
}

func NewTypedClient[T any, U any](client *p2p.Client, parser Marshaler[T, U]) *TypedClient[T, U] {
	return &TypedClient[T, U]{
		Client:    client,
		marshaler: parser,
	}
}

// AppRequest issues an arbitrary request to a node.
// [onResponse] is invoked upon an error or a response.
func (c *TypedClient[T, U]) AppRequest(
	ctx context.Context,
	nodeIDs set.Set[ids.NodeID],
	appRequestBytes []byte,
	onResponse func(
		ctx context.Context,
		nodeID ids.NodeID,
		t U,
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

	return c.Client.AppRequest(
		ctx,
		nodeIDs,
		appRequestBytes,
		onByteResponse,
	)
}
