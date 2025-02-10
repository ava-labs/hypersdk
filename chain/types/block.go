// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package types

import (
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/hypersdk/utils"
)

type Block[T canoto.FieldMaker[T]] struct {
	ParentID  ids.ID `canoto:"fixed bytes,1" json:"parent"`
	Timestamp int64  `canoto:"sint,2" json:"timestamp"`
	Height    uint64 `canoto:"fint64,3" json:"height"`

	BlockContext *block.Context `canoto:"field,4" json:"blockContext"`
	// StateRoot is the root of the post-execution state
	// of [Prnt].
	//
	// This "deferred root" design allows for merklization
	// to be done asynchronously instead of during [Build]
	// or [Verify], which reduces the amount of time we are
	// blocking the consensus engine from voting on the block,
	// starting the verification of another block, etc.
	StateRoot ids.ID `canoto:"fixed bytes,5" json:"stateRoot"`

	Txs []T `canoto:"repeated field,6" json:"txs"`

	bytes []byte
	id    ids.ID

	canotoData canotoData_Block
}

func NewBlock[T canoto.FieldMaker[T]](
	parentID ids.ID,
	timestamp int64,
	height uint64,
	txs []T,
	stateRoot ids.ID,
	blockContext *block.Context,
) *Block[T] {
	block := &Block[T]{
		ParentID:     parentID,
		Timestamp:    timestamp,
		Height:       height,
		Txs:          txs,
		StateRoot:    stateRoot,
		BlockContext: blockContext,
	}
	block.bytes = block.MarshalCanoto()
	block.id = utils.ToID(block.bytes)
	return block
}

func ParseBlock[T canoto.FieldMaker[T]](bytes []byte) (*Block[T], error) {
	block := &Block[T]{}
	err := block.UnmarshalCanoto(bytes)
	if err != nil {
		return nil, err
	}
	block.init()
	return block, nil
}

func (b *Block[T]) init() {
	if len(b.bytes) != 0 {
		return
	}
	b.bytes = b.MarshalCanoto()
	b.id = utils.ToID(b.bytes)
}

func (b *Block[T]) GetID() ids.ID {
	b.init()
	return b.id
}

func (b *Block[T]) GetParent() ids.ID {
	return b.ParentID
}

func (b *Block[T]) GetTimestamp() int64 {
	return b.Timestamp
}

func (b *Block[T]) GetBytes() []byte {
	b.init()
	return b.bytes
}

func (b *Block[T]) GetHeight() uint64 {
	return b.Height
}

func (b *Block[T]) GetContext() *block.Context {
	return b.BlockContext
}

func (b *Block[T]) GetStateRoot() ids.ID {
	return b.StateRoot
}

func (b *Block[T]) String() string {
	return fmt.Sprintf("(BlockID=%s, Height=%d, Timestamp=%d, ParentRoot=%s, NumTxs=%d, Size=%d)", b.id, b.Height, b.Timestamp, b.ParentID, len(b.Txs), len(b.bytes))
}
