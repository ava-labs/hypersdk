// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

type ExecutionResults struct {
	Results       []*Result       `json:"results"`
	UnitPrices    fees.Dimensions `json:"unitPrices"`
	UnitsConsumed fees.Dimensions `json:"unitsConsumed"`
}

func NewExecutionResults(
	results []*Result,
	unitPrices fees.Dimensions,
	unitsConsumed fees.Dimensions,
) ExecutionResults {
	return ExecutionResults{
		Results:       results,
		UnitPrices:    unitPrices,
		UnitsConsumed: unitsConsumed,
	}
}

func (e *ExecutionResults) Size() int {
	return codec.CummSize(e.Results) + fees.DimensionsLen + fees.DimensionsLen
}

func (e *ExecutionResults) Write(writer *codec.Packer) error {
	resultBytes, err := MarshalResults(e.Results)
	if err != nil {
		return err
	}
	writer.PackBytes(resultBytes)
	writer.PackFixedBytes(e.UnitPrices.Bytes())
	writer.PackFixedBytes(e.UnitsConsumed.Bytes())
	return writer.Err()
}

func (e *ExecutionResults) Marshal() ([]byte, error) {
	writer := codec.NewWriter(e.Size(), consts.MaxInt)
	if err := e.Write(writer); err != nil {
		return nil, err
	}
	return writer.Bytes(), writer.Err()
}

func (e *ExecutionResults) Read(reader *codec.Packer) error {
	var resultsMsg []byte
	reader.UnpackBytes(-1, true, &resultsMsg)
	results, err := UnmarshalResults(resultsMsg)
	if err != nil {
		return err
	}
	unitPricesBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &unitPricesBytes)
	prices, err := fees.UnpackDimensions(unitPricesBytes)
	if err != nil {
		return err
	}
	consumedBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &consumedBytes)
	consumed, err := fees.UnpackDimensions(consumedBytes)
	if err != nil {
		return err
	}
	e.Results = results
	e.UnitPrices = prices
	e.UnitsConsumed = consumed
	return nil
}

func UnmarshalExecutionResults(b []byte) (*ExecutionResults, error) {
	reader := codec.NewReader(b, consts.NetworkSizeLimit)
	e := &ExecutionResults{}
	if err := e.Read(reader); err != nil {
		return nil, err
	}
	if !reader.Empty() {
		return nil, ErrInvalidObject
	}
	return e, reader.Err()
}

type ExecutedBlock struct {
	Block *StatelessBlock `json:"block"`
	ExecutionResults
}

func NewExecutedBlock(statelessBlock *StatelessBlock, results []*Result, unitPrices fees.Dimensions, unitsConsumed fees.Dimensions) *ExecutedBlock {
	return &ExecutedBlock{
		Block: statelessBlock,
		ExecutionResults: ExecutionResults{
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

	size := codec.BytesLen(blockBytes) + e.ExecutionResults.Size()
	writer := codec.NewWriter(size, consts.MaxInt)

	writer.PackBytes(blockBytes)
	if err := e.ExecutionResults.Write(writer); err != nil {
		return nil, err
	}

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
	e := &ExecutionResults{}
	if err := e.Read(reader); err != nil {
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
		ExecutionResults: *e,
	}, nil
}

func (e *ExecutedBlock) String() string {
	return fmt.Sprintf("(Block=%s, UnitPrices=%s, UnitsConsumed=%s)", e.Block, e.UnitPrices, e.UnitsConsumed)
}
