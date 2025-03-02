// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/proto/pb/sdk"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/typedclient"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ typedclient.Marshaler[*dsmr.GetChunkRequest, Chunk[Tx], []byte]              = (*getChunkMarshaler[Tx])(nil)
	_ typedclient.Marshaler[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte] = (*getChunkSignatureMarshaler)(nil)
	_ typedclient.Marshaler[[]byte, []byte, *dsmr.ChunkCertificateGossip]          = (*chunkCertificateGossipMarshaler)(nil)
)

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

func NewGetChunkClient[T Tx](client *p2p.Client) *typedclient.TypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte] {
	return typedclient.NewTypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte](client, getChunkMarshaler[T]{})
}

func NewGetChunkSignatureClient(networkID uint32, chainID ids.ID, client *p2p.Client) *typedclient.TypedClient[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte] {
	return typedclient.NewTypedClient[*sdk.SignatureRequest, *sdk.SignatureResponse, []byte](client, getChunkSignatureMarshaler{
		networkID: networkID,
		chainID:   chainID,
	})
}

func NewChunkCertificateGossipClient(client *p2p.Client) *typedclient.TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip] {
	return typedclient.NewTypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip](client, chunkCertificateGossipMarshaler{})
}
