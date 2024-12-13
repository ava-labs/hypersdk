// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/proto/pb/sdk"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Marshaler[*dsmr.GetChunkRequest, Chunk[Tx], []byte]              = (*getChunkMarshaler[Tx])(nil)
	_ Marshaler[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte] = (*getChunkSignatureMarshaler)(nil)
	_ Marshaler[[]byte, []byte, *dsmr.ChunkCertificateGossip]          = (*chunkCertificateGossipMarshaler)(nil)
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

type getChunkSignatureMarshaler struct {
	networkID uint32
	chainID   ids.ID
}

func (getChunkSignatureMarshaler) MarshalRequest(request *sdk.SignatureRequest) ([]byte, error) {
	return proto.Marshal(request)
}

func (getChunkSignatureMarshaler) UnmarshalResponse(bytes []byte) (*sdk.SignatureResponse, error) {
	acp118Response := &sdk.SignatureResponse{}
	if err := proto.Unmarshal(bytes, acp118Response); err != nil {
		return nil, err
	}

	return acp118Response, nil
}

func (getChunkSignatureMarshaler) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}

type getChunkMarshaler[T Tx] struct{}

func (getChunkMarshaler[_]) MarshalRequest(t *dsmr.GetChunkRequest) ([]byte, error) {
	return proto.Marshal(t)
}

func (getChunkMarshaler[T]) UnmarshalResponse(bytes []byte) (Chunk[T], error) {
	response := dsmr.GetChunkResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return Chunk[T]{}, err
	}

	chunk := Chunk[T]{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: response.Chunk},
		&chunk,
	); err != nil {
		return Chunk[T]{}, err
	}

	return chunk, chunk.init()
}

func (getChunkMarshaler[_]) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}

func NewGetChunkClient[T Tx](client *p2p.Client) *TypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte] {
	return &TypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte]{
		client:    client,
		marshaler: getChunkMarshaler[T]{},
	}
}

func NewGetChunkSignatureClient(networkID uint32, chainID ids.ID, client *p2p.Client) *TypedClient[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte] {
	return &TypedClient[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte]{
		client: client,
		marshaler: getChunkSignatureMarshaler{
			networkID: networkID,
			chainID:   chainID,
		},
	}
}

func NewChunkCertificateGossipClient(client *p2p.Client) *TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip] {
	return &TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip]{
		client:    client,
		marshaler: chunkCertificateGossipMarshaler{},
	}
}
