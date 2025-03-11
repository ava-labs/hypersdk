// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"google.golang.org/protobuf/proto"
)

var (
	_ p2p.Handler = (*GetChunkHandler)(nil)
)

type GetChunkHandlerBackend interface {
	GetChunkBytes(chunkID ids.ID) ([]byte, error)
	AddChunkCert(cert *ChunkCertificate)
	CommitChunk()
}

type GetChunkHandler struct {
	backend GetChunkHandlerBackend
}

func (GetChunkHandler) AppGossip(context.Context, ids.NodeID, []byte) {}

func (g *GetChunkHandler) AppRequest(_ context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
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
	chunkBytes, err := g.backend.GetChunkBytes(chunkID)
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

type ChunkCertificateHandlerBackend interface {
	AddChunkCert(cert *ChunkCertificate)
}

type ChunkCertificateGossipHandler struct {
	backend ChunkCertificateHandlerBackend
}

func (c *ChunkCertificateGossipHandler) AppGossip(_ context.Context, _ ids.NodeID, gossipBytes []byte) {
	gossip := &dsmr.ChunkCertificateGossip{}
	if err := proto.Unmarshal(gossipBytes, gossip); err != nil {
		return
	}

	chunkCert, err := ParseChunkCertificate(gossip.ChunkCertificate)
	if err != nil {
		return
	}
	c.backend.AddChunkCert(chunkCert)
}

func (ChunkCertificateGossipHandler) AppRequest(_ context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	return nil, common.ErrTimeout
}
