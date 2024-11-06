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
	Block         *StatelessBlock `json:"block"`
	Results       []*Result       `json:"results"`
	UnitPrices    fees.Dimensions `json:"unitPrices"`
	UnitsConsumed fees.Dimensions `json:"unitsConsumed"`
}

func NewExecutedBlock(statelessBlock *StatelessBlock, results []*Result, unitPrices fees.Dimensions, unitsConsumed fees.Dimensions) *ExecutedBlock {
	return &ExecutedBlock{
		Block:         statelessBlock,
		Results:       results,
		UnitPrices:    unitPrices,
		UnitsConsumed: unitsConsumed,
	}
}

func (b *ExecutedBlock) Marshal() ([]byte, error) {
	blockBytes, err := b.Block.Marshal()
	if err != nil {
		return nil, err
	}

	size := codec.BytesLen(blockBytes) + codec.CummSize(b.Results) + fees.DimensionsLen
	writer := codec.NewWriter(size, consts.NetworkSizeLimit)

	writer.PackBytes(blockBytes)
	resultBytes, err := MarshalResults(b.Results)
	if err != nil {
		return nil, err
	}
	writer.PackBytes(resultBytes)
	writer.PackFixedBytes(b.UnitPrices.Bytes())
	writer.PackFixedBytes(b.UnitsConsumed.Bytes())

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
	var resultsMsg []byte
	reader.UnpackBytes(-1, true, &resultsMsg)
	results, err := UnmarshalResults(resultsMsg)
	if err != nil {
		return nil, err
	}
	unitPricesBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &unitPricesBytes)
	prices, err := fees.UnpackDimensions(unitPricesBytes)
	if err != nil {
		return nil, err
	}
	consumedBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &consumedBytes)
	consumed, err := fees.UnpackDimensions(consumedBytes)
	if err != nil {
		return nil, err
	}
	if !reader.Empty() {
		return nil, ErrInvalidObject
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	return NewExecutedBlock(blk, results, prices, consumed), nil
}

func (e *ExecutedBlock) String() string {
	return fmt.Sprintf("(Block=%s, UnitPrices=%s, UnitsConsumed=%s)", e.Block, e.UnitPrices, e.UnitsConsumed)
}
