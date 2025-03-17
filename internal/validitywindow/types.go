// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"github.com/ava-labs/hypersdk/internal/typedclient"
)

var _ typedclient.Marshaler[*BlockFetchRequest, *BlockFetchResponse, []byte] = (*blockFetcherMarshaler)(nil)

type BlockFetchRequest struct {
	BlockHeight  uint64 `canoto:"fint64,1"`
	MinTimestamp int64  `canoto:"fint64,2"`

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
