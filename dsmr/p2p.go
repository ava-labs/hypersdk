package dsmr

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ p2p.Handler = (*GetChunkHandler)(nil)
	_ p2p.Handler = (*GetChunkSignatureHandler[Tx])(nil)
)

type GetChunkHandler struct{}

func (g GetChunkHandler) AppGossip(context.Context, ids.NodeID, []byte) {
	return
}

func (g GetChunkHandler) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	return nil, nil
}

type GetChunkClient[T Tx] struct {
	client *p2p.Client
}

func (c GetChunkClient[T]) GetChunk(
	ctx context.Context,
	nodeID ids.NodeID,
	chunkID ids.ID,
	onChunkResponse func(context.Context, Chunk[T], error),
) error {
	request := dsmr.GetChunkRequest{
		ChunkId: chunkID[:],
	}

	requestBytes, err := proto.Marshal(&request)
	if err != nil {
		return err
	}

	return c.client.AppRequest(ctx, set.Of(nodeID), requestBytes, func(
		ctx context.Context,
		nodeID ids.NodeID,
		responseBytes []byte,
		err error,
	) {

		response := dsmr.Chunk{}
		if err := proto.Unmarshal(responseBytes, &response); err != nil {
			//TODO log
		}

		chunk := Chunk[T]{
			//Producer:    response.Producer,
			//Expiry:      response.Expiry,
			//Beneficiary: response.Beneficiary,
			//Txs:         response.Transactions,
			//Signer:    response.Signer,
			//Signature: response.Signature,
		}

		onChunkResponse(ctx, chunk, err)
	})
}

type ChunkSignature struct {
	ChunkID    ids.ID
	ProducerID ids.NodeID
	Expiry     time.Time
	Signer     [bls.PublicKeyLen]byte
	Signature  [bls.SignatureLen]byte
}

// Receives a chunk, persists it, signs it, and replies w/ a signature
// Producer sends chunks to peers for replication + collect signatures
type GetChunkSignatureHandler[T Tx] struct {
	storage Storage[T]
}

func (c *GetChunkSignatureHandler[_]) AppGossip(context.Context, ids.NodeID, []byte) {
	return
}

func (c *GetChunkSignatureHandler[T]) AppRequest(
	_ context.Context,
	_ ids.NodeID,
	_ time.Time,
	appRequestBytes []byte,
) ([]byte, *common.AppError) {
	request := &dsmr.GetChunkSignatureRequest{}
	if err := proto.Unmarshal(appRequestBytes, request); err != nil {
		panic(err)
	}

	chunk, err := parseChunkProto[T](request.Chunk)
	if err != nil {
		panic(err)
	}

	//TODO persist + sign
	//TODO conflicting?
	signature, err := c.storage.VerifyRemoteChunk(chunk)
	if err != nil {
		panic(err)
	}

	p := wrappers.Packer{MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(signature, &p); err != nil {
		panic(err)
	}

	response := &dsmr.GetChunkSignatureResponse{
		Signature: &dsmr.ChunkSignature{
			ChunkId:   chunk.id[:],
			Producer:  chunk.Producer[:],
			Expiry:    chunk.Expiry,
			Signer:    nil,
			Signature: nil,
		},
	}

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		panic(err)
	}

	return responseBytes, nil
}

type ChunkSignatureClient[T Tx] struct {
	client *p2p.Client
}

func (c ChunkSignatureClient[T]) GetChunkSignature(
	ctx context.Context,
	nodeID ids.NodeID,
	chunk Chunk[T],
	onSignatureResponse func(context.Context, ChunkSignature, error),
) error {
	request := dsmr.GetChunkSignatureRequest{}

	requestBytes, err := proto.Marshal(&request)
	if err != nil {
		return err
	}

	return c.client.AppRequest(ctx, set.Of(nodeID), requestBytes, func(
		ctx context.Context,
		nodeID ids.NodeID,
		responseBytes []byte,
		err error,
	) {

		response := dsmr.GetChunkSignatureResponse{}
		if err := proto.Unmarshal(responseBytes, &response); err != nil {
			panic(err)
		}

		chunkSignature := ChunkSignature{
			//Producer:    response.Producer,
			//Expiry:      response.Expiry,
			//Beneficiary: response.Beneficiary,
			//Txs:         response.Transactions,
			//Signer:    response.Signer,
			//Signature: response.Signature,
		}

		onSignatureResponse(ctx, chunkSignature, err)
	})
}
