// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	ErrBlocksNotFound
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
// Each request returns blocks in descending height order until:
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
	if err != nil {
		return nil, &common.AppError{
			Code:    ErrBlocksNotFound,
			Message: fmt.Sprintf("no blocks found starting at height %d: %v", request.BlockHeight, err),
		}
	}

	response := &BlockFetchResponse{Blocks: blocks}
	return response.MarshalCanoto(), nil
}

func (*BlockFetcherHandler[T]) AppGossip(_ context.Context, _ ids.NodeID, _ []byte) {}

// Client                                    Server
// |--- FetchBlocks(height=5,         ------>|
// |    minTimestamp=3)                | Get block 5 (timestamp=5)
// |                                   | Get block 4 (timestamp=4)
// |                                   | Get block 3 (timestamp=3)
// |                                   | Get block 2 (timestamp=2)
// |                                   | Stop after including block 2
// |                                   | since 2 < minTimestamp(3)
// |<-- [Block5..Block2]           --- |
func (b *BlockFetcherHandler[T]) fetchBlocks(ctx context.Context, request *BlockFetchRequest) ([][]byte, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, maxProcessingDuration)
	defer cancel()

	var (
		blocks       [][]byte
		height       = request.BlockHeight
		minTimestamp = request.MinTimestamp
	)

	for {
		block, err := b.retriever.GetBlockByHeight(timeoutCtx, height)

		switch {
		case err != nil && len(blocks) == 0:
			return nil, err
		// Handle partial responses
		case err != nil:
			return blocks, nil
		}

		blocks = append(blocks, block.GetBytes())
		height--

		if height == 0 ||
			timeoutCtx.Err() != nil ||
			block.GetTimestamp() < minTimestamp {
			return blocks, nil
		}
	}
}
