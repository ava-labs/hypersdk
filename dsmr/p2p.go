package dsmr

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ p2p.Handler = (*GetChunkHandler)(nil)
	_ p2p.Handler = (*GetChunkSignatureHandler)(nil)
)

type GetChunkHandler struct{}

func (g GetChunkHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, gossipBytes []byte) {
	return
}

func (g GetChunkHandler) AppRequest(ctx context.Context, nodeID ids.NodeID, deadline time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	return nil, nil
}

// Receives a chunk, persists it, signs it, and replies w/ a signature
// Producer sends chunks to peers for replication + collect signatures
type GetChunkSignatureHandler struct {
}

func (g GetChunkSignatureHandler) AppGossip(context.Context, ids.NodeID, []byte) {
	return
}

func (g GetChunkSignatureHandler) AppRequest(_ context.Context, _ ids.NodeID, _ time.Time, appRequestBytes []byte) ([]byte, *common.AppError) {
	request := &dsmr.Chunk{}
	if err := proto.Unmarshal(appRequestBytes, request); err != nil {
		panic(err)
	}

	//TODO persist + sign
	//TODO conflicting?

	response := &dsmr.ChunkSignature{}
	responseBytes, err := proto.Marshal(response)
	if err != nil {
		panic(err)
	}

	return responseBytes, nil
}
