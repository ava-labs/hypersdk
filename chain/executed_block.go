// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

type ExecutedBlock struct {
	Block            *StatelessBlock   `json:"block"`
	ExecutionResults *ExecutionResults `json:"results"`
}

func NewExecutedBlock(statelessBlock *StatelessBlock, results []*Result, unitPrices fees.Dimensions, unitsConsumed fees.Dimensions) *ExecutedBlock {
	return &ExecutedBlock{
		Block: statelessBlock,
		ExecutionResults: &ExecutionResults{
			Results:       results,
			UnitPrices:    unitPrices,
			UnitsConsumed: unitsConsumed,
		},
	}
}

func (e *ExecutedBlock) Marshal() ([]byte, error) {
	blockBytes, err := e.Block.Marshal()
	if err != nil {
		return nil, err
	}

	executionResultsBytes := e.ExecutionResults.Marshal()
	size := codec.BytesLen(blockBytes) + codec.BytesLen(executionResultsBytes)
	writer := codec.NewWriter(size, consts.MaxInt)

	writer.PackBytes(blockBytes)
	writer.PackBytes(executionResultsBytes)

	return writer.Bytes(), writer.Err()
}

func UnmarshalExecutedBlock(bytes []byte, parser Parser) (*ExecutedBlock, error) {
	reader := codec.NewReader(bytes, consts.NetworkSizeLimit)

	var blkMsg []byte
	reader.UnpackBytes(-1, true, &blkMsg)
	blk, err := UnmarshalBlock(blkMsg, parser)
	if err != nil {
		return nil, err
	}
	var executionResultsBytes []byte
	reader.UnpackBytes(-1, true, &executionResultsBytes)
	results, err := ParseExecutionResults(executionResultsBytes)
	if err != nil {
		return nil, err
	}

	if !reader.Empty() {
		return nil, ErrInvalidObject
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	return &ExecutedBlock{
		Block:            blk,
		ExecutionResults: results,
	}, nil
}

func (e *ExecutedBlock) String() string {
	return fmt.Sprintf("(Block=%s, UnitPrices=%s, UnitsConsumed=%s)", e.Block, e.ExecutionResults.UnitPrices, e.ExecutionResults.UnitsConsumed)
}
