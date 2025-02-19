// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
)

// maxProcessingDuration of the request
const maxProcessingDuration = 50 * time.Millisecond

const (
	ErrCodeUnmarshal = iota + 1
	ErrCodeRetrieval
)

var errUnmarshalRequest = &common.AppError{
	Code:    ErrCodeUnmarshal,
	Message: "failed to unmarshal request",
}

// HandlerBlock is block returned by the handler
type HandlerBlock interface {
	GetTimestamp() int64
	GetBytes() []byte
}

// BlockRetriever defines operations needed by a node serving blocks to peers
// Generic T must at least implement HandlerBlock
type BlockRetriever[T HandlerBlock] interface {
	GetBlockByHeight(ctx context.Context, blockHeight uint64) (T, error)
}

// BlockFetcherHandler handles incoming block fetch requests with a time limit
// Each request returns blocks in descending nextHeight order until:
// - maxProcessingDuration is reached
// - minTimestamp is reached
type BlockFetcherHandler[T HandlerBlock] struct {
	retriever BlockRetriever[T]
}

func NewBlockFetcherHandler[T HandlerBlock](retriever BlockRetriever[T]) *BlockFetcherHandler[T] {
	return &BlockFetcherHandler[T]{retriever: retriever}
}

func (b *BlockFetcherHandler[T]) AppRequest(
	ctx context.Context,
	_ ids.NodeID,
	_ time.Time,
	requestBytes []byte,
) ([]byte, *common.AppError) {
	request := new(BlockFetchRequest)
	if err := request.UnmarshalCanoto(requestBytes); err != nil {
		return nil, errUnmarshalRequest
	}

	blocks, err := b.fetchBlocks(ctx, request)
	response := &BlockFetchResponse{Blocks: blocks}

	// If we have any blocks, return them even if there was an error (partial response)
	if len(blocks) > 0 {
		return response.MarshalCanoto(), nil
	}

	if err != nil {
		return nil, &common.AppError{
			Code:    ErrCodeRetrieval,
			Message: fmt.Errorf("failed to fetch blocks: %w", err).Error(),
		}
	}

	return response.MarshalCanoto(), nil
}

func (*BlockFetcherHandler[T]) AppGossip(_ context.Context, _ ids.NodeID, _ []byte) {}

// Client                              Server
// |--- FetchBlocks(nextHeight=100, ------>|
// |    minTimestamp=X)                | - Get block 100
// |                                   | - Get block 99
// |                                   | - Get block 98
// |                                   | - Until minTimestamp < X
// |<-- [Block100,Block99,Block98] --- |
func (b *BlockFetcherHandler[T]) fetchBlocks(ctx context.Context, request *BlockFetchRequest) ([][]byte, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, maxProcessingDuration)
	defer cancel()

	var (
		blocks       [][]byte
		height       = request.BlockHeight
		minTimestamp = request.MinTimestamp
	)

	for height > 0 {
		select {
		case <-timeoutCtx.Done():
			return blocks, timeoutCtx.Err()
		default:
			block, err := b.retriever.GetBlockByHeight(ctx, height)
			if err != nil {
				return blocks, err
			}
			if block.GetTimestamp() < minTimestamp {
				return blocks, nil
			}
			blocks = append(blocks, block.GetBytes())
			height--
		}
	}
	return blocks, nil
}
