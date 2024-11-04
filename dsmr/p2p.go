// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	// TODO error codes
	ErrChunkNotAvailable = &common.AppError{
		Code:    1,
		Message: "chunk is not available",
	}

	ErrDuplicateChunk = &common.AppError{
		Code:    2,
		Message: "chunk is already available",
	}

	ErrInvalidChunk = &common.AppError{
		Code:    3,
		Message: "invalid chunk",
	}

	_ p2p.Handler = (*GetChunkHandler[Tx])(nil)
	_ p2p.Handler = (*GetChunkSignatureHandler[Tx])(nil)
	_ p2p.Handler = (*ChunkCertificateGossipHandler[Tx])(nil)
)

type GetChunkHandler[T Tx] struct {
	storage *chunkStorage[T]
}

func (*GetChunkHandler[_]) AppGossip(context.Context, ids.NodeID, []byte) {}

// TODO can only get available chunks that have not expired/executed chunks near
// tip
func (g *GetChunkHandler[T]) AppRequest(_ context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	request := dsmr.GetChunkRequest{}
	if err := proto.Unmarshal(requestBytes, &request); err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	chunkID, err := ids.ToID(request.ChunkId)
	if err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	// TODO check chunk status?
	chunkBytes, available, err := g.storage.GetChunkBytes(request.Expiry, chunkID)
	if err != nil && errors.Is(err, database.ErrNotFound) {
		return nil, ErrChunkNotAvailable
	}
	if err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	if !available {
		return nil, ErrChunkNotAvailable
	}

	chunk, err := ParseChunk[T](chunkBytes)
	if err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
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
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	return responseBytes, nil
}

// TODO rename, this is a storage type
type ChunkSignature struct {
	ChunkID    ids.ID
	ProducerID ids.NodeID
	Expiry     time.Time
	Signer     [bls.PublicKeyLen]byte
	Signature  [bls.SignatureLen]byte
}

type GetChunkSignatureHandler[T Tx] struct {
	sk       *bls.SecretKey
	verifier Verifier[T]
	storage  *chunkStorage[T]
}

func (*GetChunkSignatureHandler[T]) AppGossip(context.Context, ids.NodeID, []byte) {}

func (g *GetChunkSignatureHandler[T]) AppRequest(
	_ context.Context,
	_ ids.NodeID,
	_ time.Time,
	requestBytes []byte,
) ([]byte, *common.AppError) {
	request := &dsmr.GetChunkSignatureRequest{}
	if err := proto.Unmarshal(requestBytes, request); err != nil {
		return nil, &common.AppError{
			Code:    p2p.ErrUnexpected.Code,
			Message: err.Error(),
		}
	}

	chunk, err := newChunkFromProto[T](request.Chunk)
	if err != nil {
		return nil, &common.AppError{
			Code:    p2p.ErrUnexpected.Code,
			Message: err.Error(),
		}
	}

	if err := g.verifier.Verify(chunk); err != nil {
		return nil, ErrInvalidChunk
	}

	_, accepted, err := g.storage.GetChunkBytes(chunk.Expiry, chunk.id)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, &common.AppError{
			Code:    p2p.ErrUnexpected.Code,
			Message: err.Error(),
		}
	}

	if accepted {
		// Don't sign a chunk that is already marked as accepted
		return nil, ErrDuplicateChunk
	}

	if _, err := g.storage.VerifyRemoteChunk(chunk); err != nil {
		return nil, &common.AppError{
			Code:    p2p.ErrUnexpected.Code,
			Message: err.Error(),
		}
	}

	response := &dsmr.GetChunkSignatureResponse{
		ChunkId:   chunk.id[:],
		Producer:  chunk.Producer[:],
		Expiry:    chunk.Expiry,
		Signer:    bls.PublicKeyToCompressedBytes(bls.PublicFromSecretKey(g.sk)),
		Signature: bls.SignatureToBytes(bls.Sign(g.sk, chunk.bytes)),
	}

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		return nil, &common.AppError{
			Code:    p2p.ErrUnexpected.Code,
			Message: err.Error(),
		}
	}

	return responseBytes, nil
}

type ChunkCertificateGossipHandler[T Tx] struct {
	storage *chunkStorage[T]
}

func (c ChunkCertificateGossipHandler[_]) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	gossip := &dsmr.ChunkCertificateGossip{}
	if err := proto.Unmarshal(gossipBytes, gossip); err != nil {
		return
	}

	chunkID, err := ids.ToID(gossip.ChunkCertificate.ChunkId)
	if err != nil {
		return
	}

	chunkCert := &ChunkCertificate{
		ChunkID:   chunkID,
		Expiry:    gossip.ChunkCertificate.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	if err := c.storage.SetChunkCert(chunkID, chunkCert); err != nil {
		return
	}
}

func (c ChunkCertificateGossipHandler[_]) AppRequest(context.Context, ids.NodeID, time.Time, []byte) ([]byte, *common.AppError) {
	return nil, common.ErrTimeout
}
