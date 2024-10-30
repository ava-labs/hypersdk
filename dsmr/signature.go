package dsmr

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var _ acp118.Verifier = (*ChunkSignatureVerifier[Tx])(nil)

type ChunkSignatureVerifier[T Tx] struct {
	storage *ChunkStorage[T]
}

func (c ChunkSignatureVerifier[T]) Verify(
	_ context.Context,
	message *warp.UnsignedMessage,
	_ []byte,
) *common.AppError {
	request := &dsmr.GetChunkSignatureRequest{}
	if err := proto.Unmarshal(message.Payload, request); err != nil {
		panic(err)
	}

	chunk, err := newChunkFromProto[T](request.Chunk)
	if err != nil {
		panic(err)
	}

	//TODO persist + sign
	//TODO conflicting?
	_, err = c.storage.VerifyRemoteChunk(chunk)
	if err != nil {
		panic(err)
	}

	return nil
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
	txs, err := marshalTxs[T](chunk.Txs)
	if err != nil {
		return err
	}

	request := &dsmr.GetChunkSignatureRequest{
		Chunk: &dsmr.Chunk{
			Producer:     chunk.Producer[:],
			Expiry:       chunk.Expiry,
			Beneficiary:  chunk.Beneficiary[:],
			Transactions: txs,
			Signer:       nil,
			Signature:    nil,
		},
	}

	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return err
	}

	return c.client.AppRequest(ctx, set.Of(nodeID), requestBytes, func(
		ctx context.Context,
		nodeID ids.NodeID,
		responseBytes []byte,
		err error,
	) {

		response := &dsmr.GetChunkSignatureResponse{}
		if err := proto.Unmarshal(responseBytes, response); err != nil {
			panic(err)
		}

		chunkSignature := ChunkSignature{
			ChunkID:    ids.ID{},
			ProducerID: ids.NodeID{},
			Expiry:     time.Time{},
			Signer:     [48]byte{},
			Signature:  [96]byte{},
		}

		onSignatureResponse(ctx, chunkSignature, err)
	})
}
