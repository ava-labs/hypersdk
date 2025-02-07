// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/x/dsmr"
)

var _ dsmr.Marshaler[*BlockFetchRequest, *BlockFetchResponse, []byte] = (*blockFetcherMarshaler)(nil)

type Block interface {
	fmt.Stringer
	GetID() ids.ID
	GetHeight() uint64
	GetParent() ids.ID
	GetTimestamp() int64
	GetBytes() []byte
}

// BlockParser handles parsing and storing blocks received from peers
type BlockParser[T Block] interface {
	// ParseBlock converts raw block bytes into a block type
	ParseBlock(ctx context.Context, blockBytes []byte) (T, error)

	// WriteBlock stores the parsed block
	WriteBlock(ctx context.Context, block T) error
}

type BlockFetcher[T Block] interface {
	// FetchBlocks sends fetch request to peers starting from block until it fills time validity window
	FetchBlocks(ctx context.Context, block T, minTimestamp int64) error
}

// BlockRetriever defines operations needed by a node serving blocks to peers
type BlockRetriever[T Block] interface {
	GetBlockByHeight(ctx context.Context, blockHeight uint64) (T, error)
}

type BlockFetchRequest struct {
	BlockHeight  uint64 `canoto:"int,1"`
	MinTimestamp int64  `canoto:"int,2"`

	canotoData canotoData_BlockFetchRequest
}

type BlockFetchResponse struct {
	Blocks [][]byte `canoto:"repeated bytes,1"`

	canotoData canotoData_BlockFetchResponse
}

type blockFetcherMarshaler struct{}

func (*blockFetcherMarshaler) MarshalRequest(request *BlockFetchRequest) ([]byte, error) {
	return request.MarshalCanoto(), nil
}

func (*blockFetcherMarshaler) UnmarshalResponse(bytes []byte) (*BlockFetchResponse, error) {
	resp := new(BlockFetchResponse)
	return resp, resp.UnmarshalCanoto(bytes)
}

func (*blockFetcherMarshaler) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}
