// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/hypersdk/fees"
)

type ExecutedBlock struct {
	Block            *StatelessBlock   `canoto:"pointer,1" json:"block"`
	ExecutionResults *ExecutionResults `canoto:"pointer,2" json:"results"`

	canotoData canotoData_ExecutedBlock
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
	return e.MarshalCanoto(), nil
}

func UnmarshalExecutedBlock(bytes []byte, parser Parser) (*ExecutedBlock, error) {
	r := canoto.Reader{
		B: bytes,
		Context: &TxSerializer{
			ActionRegistry: parser.ActionCodec(),
			AuthRegistry:   parser.AuthCodec(),
		},
	}
	b := new(ExecutedBlock)
	if err := b.UnmarshalCanotoFrom(r); err != nil {
		return nil, err
	}
	b.CalculateCanotoCache()
	return b, nil
}

func (e *ExecutedBlock) String() string {
	return fmt.Sprintf("(Block=%s, UnitPrices=%s, UnitsConsumed=%s)", e.Block, e.ExecutionResults.UnitPrices, e.ExecutionResults.UnitsConsumed)
}
