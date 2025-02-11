// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/hypersdk/utils"
)

type Block[T Action[T], A Auth[A]] struct {
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

	Txs []*Transaction[T, A] `canoto:"repeated field,6" json:"txs"`

	bytes []byte
	id    ids.ID

	canotoData canotoData_Block
}

func NewBlock[T Action[T], A Auth[A]](
	parentID ids.ID,
	timestamp int64,
	height uint64,
	txs []*Transaction[T, A],
	stateRoot ids.ID,
	blockContext *block.Context,
) *Block[T, A] {
	block := &Block[T, A]{
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

func ParseBlock[T Action[T], A Auth[A]](bytes []byte) (*Block[T, A], error) {
	block := &Block[T, A]{}
	err := block.UnmarshalCanoto(bytes)
	if err != nil {
		return nil, err
	}
	block.init()
	return block, nil
}

func (b *Block[T, A]) init() {
	if len(b.bytes) != 0 {
		return
	}
	b.bytes = b.MarshalCanoto()
	b.id = utils.ToID(b.bytes)
}

func (b *Block[T, A]) GetID() ids.ID {
	b.init()
	return b.id
}

func (b *Block[T, A]) GetParent() ids.ID {
	return b.ParentID
}

func (b *Block[T, A]) GetTimestamp() int64 {
	return b.Timestamp
}

func (b *Block[T, A]) GetBytes() []byte {
	b.init()
	return b.bytes
}

func (b *Block[T, A]) GetHeight() uint64 {
	return b.Height
}

func (b *Block[T, A]) GetContext() *block.Context {
	return b.BlockContext
}

func (b *Block[T, A]) GetStateRoot() ids.ID {
	return b.StateRoot
}

func (b *Block[T, A]) String() string {
	return fmt.Sprintf("(BlockID=%s, Height=%d, Timestamp=%d, ParentRoot=%s, NumTxs=%d, Size=%d)", b.id, b.Height, b.Timestamp, b.ParentID, len(b.Txs), len(b.bytes))
}

func NewGenesisBlock[T Action[T], A Auth[A]](root ids.ID) (*ExecutionBlock[T, A], error) {
	// We set the genesis block timestamp to be after the ProposerVM fork activation.
	//
	// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
	// where the child timestamp is rounded down to the nearest second (which may be before
	// the timestamp of its parent, which is denoted in milliseconds).
	//
	// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
	// .../vms/proposervm/pre_fork_block.go#L201
	sb := NewBlock[T, A](
		ids.Empty,
		time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		0,
		nil,
		root, // StateRoot should include all allocates made when loading the genesis file
		nil,
	)
	return NewExecutionBlock(sb), nil
}
