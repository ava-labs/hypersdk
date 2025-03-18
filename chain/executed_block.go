// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"

	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

var _ validitywindow.ExecutionBlock[*Transaction] = (*ExecutedBlock)(nil)

// ExecutedBlock includes the block and its execution results.
// This type is used internally to the node and never sent/received
// over the wire, so it is not subject to the network max message size
// limitations and it is not necessary to check that the fields are
// non-nil because we will always populate them with non-nil values.
type ExecutedBlock struct {
	Block            *ExecutionBlock   `canoto:"pointer,1" json:"block"`
	ExecutionResults *ExecutionResults `canoto:"pointer,2" json:"results"`

	bytes []byte

	canotoData canotoData_ExecutedBlock
}

func NewExecutedBlock(block *ExecutionBlock, results []*Result, unitPrices fees.Dimensions, unitsConsumed fees.Dimensions) *ExecutedBlock {
	eb := &ExecutedBlock{
		Block: block,
		ExecutionResults: &ExecutionResults{
			Results:       results,
			UnitPrices:    unitPrices,
			UnitsConsumed: unitsConsumed,
		},
	}
	eb.bytes = eb.MarshalCanoto()
	return eb
}

func (e *ExecutedBlock) GetID() ids.ID     { return e.Block.id }
func (e *ExecutedBlock) GetParent() ids.ID { return e.Block.Prnt }
func (e *ExecutedBlock) GetHeight() uint64 { return e.Block.GetHeight() }
func (e *ExecutedBlock) GetBytes() []byte  { return e.bytes }

func (e *ExecutedBlock) GetTimestamp() int64 {
	return e.Block.Tmstmp
}
func (e *ExecutedBlock) GetContainers() []*Transaction {
	return e.Block.GetContainers()
}
func (e *ExecutedBlock) Contains(txID ids.ID) bool {
	return e.Block.Contains(txID)
}

type ExecutedBlockParser struct {
	parser Parser
}

func NewExecutedBlockParser(parser Parser) *ExecutedBlockParser {
	return &ExecutedBlockParser{parser: parser}
}

func (e *ExecutedBlockParser) ParseBlock(_ context.Context, bytes []byte) (*ExecutedBlock, error) {
	return UnmarshalExecutedBlock(bytes, e.parser)
}

func UnmarshalExecutedBlock(bytes []byte, parser Parser) (*ExecutedBlock, error) {
	r := canoto.Reader{
		B:       bytes,
		Context: parser,
	}
	b := new(ExecutedBlock)
	if err := b.UnmarshalCanotoFrom(r); err != nil {
		return nil, err
	}
	b.CalculateCanotoCache()
	b.bytes = bytes
	return b, nil
}

func (e *ExecutedBlock) String() string {
	return fmt.Sprintf("(Block=%s, UnitPrices=%s, UnitsConsumed=%s)", e.Block, e.ExecutionResults.UnitPrices, e.ExecutionResults.UnitsConsumed)
}
