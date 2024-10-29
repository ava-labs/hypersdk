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
	_ p2p.Handler = (*GetChunkHandler[Tx])(nil)
	_ p2p.Handler = (*GetChunkSignatureHandler[Tx])(nil)
)

type GetChunkHandler[T Tx] struct {
	storage *chunkStorage[T]
}

func (g *GetChunkHandler[_]) AppGossip(context.Context, ids.NodeID, []byte) {
	return
}

func (g *GetChunkHandler[T]) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	request := dsmr.GetChunkRequest{}
	if err := proto.Unmarshal(requestBytes, &request); err != nil {
		panic(err)
	}

	chunkID, err := ids.ToID(request.ChunkId)
	if err != nil {
		panic(err)
	}

	chunkBytes, err := g.storage.GetChunkBytes(request.Expiry, chunkID)
	if err != nil {
		panic(err)
	}

	chunk, err := ParseChunk[T](chunkBytes)
	if err != nil {
		panic(err)
	}

	txs := make([]*dsmr.Transaction, 0, len(chunk.Txs))
	for _, tx := range chunk.Txs {
		packer := &wrappers.Packer{MaxSize: consts.NetworkSizeLimit}
		if err := codec.LinearCodec.MarshalInto(tx, packer); err != nil {
			panic(err)
		}

		txs = append(txs, &dsmr.Transaction{Bytes: packer.Bytes})
	}

	response := &dsmr.GetChunkResponse{
		Chunk: &dsmr.Chunk{
			Producer:     chunk.Producer[:],
			Expiry:       chunk.Expiry,
			Beneficiary:  chunk.Beneficiary[:],
			Transactions: txs,
			Signer:       chunk.Signer[:],
			Signature:    chunk.Signature[:],
		},
	}

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		panic(err)
	}

	return responseBytes, nil
}

type GetChunkClient[T Tx] struct {
	client *p2p.Client
}

func (c GetChunkClient[T]) GetChunk(
	ctx context.Context,
	nodeID ids.NodeID,
	chunkID ids.ID,
	expiry time.Time,
	onChunkResponse func(context.Context, Chunk[T], error),
) error {
	request := dsmr.GetChunkRequest{
		ChunkId: chunkID[:],
		Expiry:  expiry.Unix(),
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

		response := dsmr.GetChunkResponse{}
		if err := proto.Unmarshal(responseBytes, &response); err != nil {
			panic(err)
		}

		chunk, err := parseChunkProto[T](response.Chunk)
		if err != nil {
			panic(err)
		}

		onChunkResponse(ctx, chunk, err)
	})
}

// TODO rename, this is a storage type
type ChunkSignature struct {
	ChunkID    ids.ID
	ProducerID ids.NodeID
	Expiry     time.Time
	Signer     [bls.PublicKeyLen]byte
	Signature  [bls.SignatureLen]byte
}

type GetChunkSignatureHandler[T Tx] struct{}

func (g GetChunkSignatureHandler[T]) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	//TODO implement me
	panic("implement me")
}

func (g GetChunkSignatureHandler[T]) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	//TODO implement me
	panic("implement me")
}
