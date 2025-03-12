// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"google.golang.org/protobuf/proto"
)

var (
	_ p2p.Handler     = (*GetChunkHandler)(nil)
	_ p2p.Handler     = (*ChunkCertificateGossipHandler)(nil)
	_ acp118.Verifier = (*ChunkSignatureRequestVerifier)(nil)
)

type HandlerBackend interface {
	GetChunkBytes(ctx context.Context, chunkRef *ChunkReference) ([]byte, error)
	AddChunkCert(ctx context.Context, cert *ChunkCertificate) error
	CommitChunk(ctx context.Context, chunk *Chunk) error
}

func RegisterAppHandlers(
	backend HandlerBackend,
	warpSigner warp.Signer,
	network *p2p.Network,
	getChunkProtocolID uint64,
	broadcastChunkCertProtocolID uint64,
	getChunkSignatureProtocolID uint64,
) error {
	getChunkHandler := &GetChunkHandler{backend: backend}
	chunkCertGossipHandler := &ChunkCertificateGossipHandler{backend: backend}
	chunkSignatureRequestVerifier := &ChunkSignatureRequestVerifier{backend: backend}
	acp118Handler := acp118.NewHandler(chunkSignatureRequestVerifier, warpSigner)

	return errors.Join(
		network.AddHandler(
			getChunkProtocolID,
			getChunkHandler,
		),
		network.AddHandler(
			broadcastChunkCertProtocolID,
			chunkCertGossipHandler,
		),
		network.AddHandler(
			getChunkSignatureProtocolID,
			acp118Handler,
		),
	)
}

type GetChunkHandler struct {
	backend HandlerBackend
}

func (GetChunkHandler) AppGossip(context.Context, ids.NodeID, []byte) {}

func (g *GetChunkHandler) AppRequest(ctx context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	request := dsmr.GetChunkRequest{}
	if err := proto.Unmarshal(requestBytes, &request); err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	chunkRef, err := ParseChunkReference(request.ChunkRef)
	if err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}
	chunkBytes, err := g.backend.GetChunkBytes(ctx, chunkRef)
	if err != nil {
		return nil, &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}
	response := &dsmr.GetChunkResponse{
		Chunk: chunkBytes,
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

type ChunkCertificateGossipHandler struct {
	backend HandlerBackend
}

func (c *ChunkCertificateGossipHandler) AppGossip(ctx context.Context, _ ids.NodeID, gossipBytes []byte) {
	gossip := &dsmr.ChunkCertificateGossip{}
	if err := proto.Unmarshal(gossipBytes, gossip); err != nil {
		return
	}

	chunkCert, err := ParseChunkCertificate(gossip.ChunkCertificate)
	if err != nil {
		return
	}
	if err := c.backend.AddChunkCert(ctx, chunkCert); err != nil {
		return
	}
}

func (ChunkCertificateGossipHandler) AppRequest(_ context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	return nil, common.ErrTimeout
}

type ChunkSignatureRequestVerifier struct {
	backend HandlerBackend
}

func (c ChunkSignatureRequestVerifier) Verify(
	ctx context.Context,
	unsignedMsg *warp.UnsignedMessage,
	justification []byte,
) *common.AppError {
	chunkRef, err := ParseChunkReference(unsignedMsg.Payload)
	if err != nil {
		return &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}

	chunk, err := ParseUnsignedChunk(justification)
	if err != nil {
		return &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}
	if chunk.id != chunkRef.ChunkID {
		return &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: "invalid request chunkID mismatch",
		}
	}

	err = c.backend.CommitChunk(ctx, chunk)
	if err != nil {
		return &common.AppError{
			Code:    common.ErrUndefined.Code,
			Message: err.Error(),
		}
	}
	return nil
}
