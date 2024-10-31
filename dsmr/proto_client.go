package dsmr

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

type marshaler[T any, U any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
}

type TypedClient[T any, U any] struct {
	client    *p2p.Client
	marshaler marshaler[T, U]
}

// TODO merge upstream into avalanchego
func newTypedClient[T any, U any](client *p2p.Client, marshaler marshaler[T, U]) *TypedClient[T, U] {
	return &TypedClient[T, U]{
		client:    client,
		marshaler: marshaler,
	}
}

// AppRequest issues an arbitrary request to a node.
// [onResponse] is invoked upon an error or a response.
func (t *TypedClient[T, U]) AppRequest(
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
		response, parseErr := t.marshaler.UnmarshalResponse(responseBytes)
		if parseErr != nil {
			//TODO how do we handle this?
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
		nodeIDs,
		requestBytes,
		onByteResponse,
	)
}

type getChunkSignatureMarshaler struct{}

func (g getChunkSignatureMarshaler) MarshalRequest(t *dsmr.GetChunkSignatureRequest) ([]byte, error) {
	return proto.Marshal(t)
}

func (g getChunkSignatureMarshaler) UnmarshalResponse(bytes []byte) (*dsmr.GetChunkSignatureResponse, error) {
	response := dsmr.GetChunkSignatureResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

type getChunkMarshaler struct{}

func (g getChunkMarshaler) MarshalRequest(t *dsmr.GetChunkRequest) ([]byte, error) {
	return proto.Marshal(t)
}

func (g getChunkMarshaler) UnmarshalResponse(bytes []byte) (*dsmr.GetChunkResponse, error) {
	response := dsmr.GetChunkResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func NewGetChunkClient(client *p2p.Client) *TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse] {
	return &TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse]{
		client:    client,
		marshaler: getChunkMarshaler{},
	}
}

func NewGetChunkSignatureClient(client *p2p.Client) *TypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse] {
	return &TypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse]{
		client:    client,
		marshaler: getChunkSignatureMarshaler{},
	}
}
