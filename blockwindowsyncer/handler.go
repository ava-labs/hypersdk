// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
)

const (
	// Maximum number of blocks that can be returned in a single response
	maxBlocksPerResponse = 100
)

const (
	ErrCodeUnmarshal = iota + 1
	ErrCodeRetrieval
)

var errUnmarshalRequest = &common.AppError{Code: ErrCodeUnmarshal, Message: "failed to unmarshal request"}

// BlockFetcherHandler handles incoming block fetch requests
// it retrieves blocks from memory or disk
type BlockFetcherHandler[T Block] struct {
	retriever BlockRetriever[T]
}

func NewBlockFetcherHandler[T Block](retriever BlockRetriever[T]) *BlockFetcherHandler[T] {
	return &BlockFetcherHandler[T]{retriever: retriever}
}

func (b *BlockFetcherHandler[T]) AppRequest(ctx context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	request := new(BlockFetchRequest)
	err := request.UnmarshalCanoto(requestBytes)
	if err != nil {
		return nil, errUnmarshalRequest
	}

	blocks, err := b.fetchBlocks(ctx, request)
	if err != nil {
		return nil, &common.AppError{Code: ErrCodeRetrieval, Message: fmt.Errorf("failed to fetch blocks: %w", err).Error()}
	}

	response := &BlockFetchResponse{Blocks: blocks}
	responseBytes := response.MarshalCanoto()

	return responseBytes, nil
}

func (*BlockFetcherHandler[T]) AppGossip(_ context.Context, _ ids.NodeID, _ []byte) {}

// Client                              Server
// |--- FetchBlocks(height=100, ------>|
// |    minTimestamp=X)                | - Get block 100
// |                                   | - Get block 99
// |                                   | - Get block 98
// |                                   | - Until timestamp < X
// |<-- [Block100,Block99,Block98] --- |
func (b *BlockFetcherHandler[T]) fetchBlocks(ctx context.Context, request *BlockFetchRequest) ([][]byte, error) {
	var (
		blocks       [][]byte
		height       = request.BlockHeight
		minTimestamp = request.MinTimestamp
	)

	for height > 0 && len(blocks) < maxBlocksPerResponse {
		// check ctx cancellation periodically
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		block, blkErr := b.retriever.GetBlockByHeight(ctx, height)
		if blkErr != nil {
			return nil, blkErr
		}

		if block.GetTimestamp() < minTimestamp {
			break
		}

		blocks = append(blocks, block.GetBytes())
		height--
	}

	return blocks, nil
}
