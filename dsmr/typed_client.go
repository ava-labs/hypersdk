// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Marshaler[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse, []byte]                   = (*getChunkMarshaler)(nil)
	_ Marshaler[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse, []byte] = (*getChunkSignatureMarshaler)(nil)
	_ Marshaler[[]byte, []byte, *dsmr.ChunkCertificateGossip]                            = (*chunkCertificateGossipMarshaler)(nil)
)

type Marshaler[T any, U any, V any] interface {
	MarshalRequest(T) ([]byte, error)
	UnmarshalResponse([]byte) (U, error)
	MarshalGossip(V) ([]byte, error)
}

// TODO merge upstream into avalanchego
type TypedClient[T any, U any, V any] struct {
	client    *p2p.Client
	marshaler Marshaler[T, U, V]
}

func (t *TypedClient[T, U, _]) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	request T,
	onResponse func(ctx context.Context, nodeID ids.NodeID, response U, err error),
) error {
	onByteResponse := func(ctx context.Context, nodeID ids.NodeID, responseBytes []byte, err error) {
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
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	return t.client.AppGossip(
		ctx,
		common.SendConfig{
			Validators: 100,
		},
		gossipBytes,
	)
}

type chunkCertificateGossipMarshaler struct{}

func (chunkCertificateGossipMarshaler) MarshalRequest(bytes []byte) ([]byte, error) {
	return bytes, nil
}

func (chunkCertificateGossipMarshaler) UnmarshalResponse(bytes []byte) ([]byte, error) {
	return bytes, nil
}

func (chunkCertificateGossipMarshaler) MarshalGossip(gossip *dsmr.ChunkCertificateGossip) ([]byte, error) {
	return proto.Marshal(gossip)
}

type getChunkSignatureMarshaler struct{}

func (getChunkSignatureMarshaler) MarshalRequest(request *dsmr.GetChunkSignatureRequest) ([]byte, error) {
	return proto.Marshal(request)
}

func (getChunkSignatureMarshaler) UnmarshalResponse(bytes []byte) (*dsmr.GetChunkSignatureResponse, error) {
	response := dsmr.GetChunkSignatureResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (getChunkSignatureMarshaler) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}

type getChunkMarshaler struct{}

func (getChunkMarshaler) MarshalRequest(t *dsmr.GetChunkRequest) ([]byte, error) {
	return proto.Marshal(t)
}

func (getChunkMarshaler) UnmarshalResponse(bytes []byte) (*dsmr.GetChunkResponse, error) {
	response := dsmr.GetChunkResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (getChunkMarshaler) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}

func NewGetChunkClient(client *p2p.Client) *TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse, []byte] {
	return &TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse, []byte]{
		client:    client,
		marshaler: getChunkMarshaler{},
	}
}

func NewGetChunkSignatureClient(client *p2p.Client) *TypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse, []byte] {
	return &TypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse, []byte]{
		client:    client,
		marshaler: getChunkSignatureMarshaler{},
	}
}

func NewChunkCertificateGossipClient(client *p2p.Client) *TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip] {
	return &TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip]{
		client:    client,
		marshaler: chunkCertificateGossipMarshaler{},
	}
}
