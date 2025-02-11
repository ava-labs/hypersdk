// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"
	"time"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"

	"github.com/ava-labs/hypersdk/utils"
)

type Block struct {
	Prnt   ids.ID `canoto:"fixed bytes,1" json:"parent"`
	Tmstmp int64  `canoto:"sint,2" json:"timestamp"`
	Hght   uint64 `canoto:"fint64,3" json:"height"`

	BlockContext *block.Context `canoto:"pointer,4" json:"blockContext"`

	Txs []*Transaction `canoto:"repeated pointer,5" json:"txs"`

	// StateRoot is the root of the post-execution state
	// of [Prnt].
	//
	// This "deferred root" design allows for merklization
	// to be done asynchronously instead of during [Build]
	// or [Verify], which reduces the amount of time we are
	// blocking the consensus engine from voting on the block,
	// starting the verification of another block, etc.
	StateRoot ids.ID `canoto:"fixed bytes,6" json:"stateRoot"`

	canotoData canotoData_Block
}

type StatelessBlock struct {
	Block

	bytes []byte
	id    ids.ID
}

func NewStatelessBlock(
	parentID ids.ID,
	timestamp int64,
	height uint64,
	txs []*Transaction,
	stateRoot ids.ID,
	blockContext *block.Context,
) (*StatelessBlock, error) {
	block := &StatelessBlock{
		Block: Block{
			Prnt:         parentID,
			Tmstmp:       timestamp,
			Hght:         height,
			Txs:          txs,
			StateRoot:    stateRoot,
			BlockContext: blockContext,
		},
	}
	blkBytes := block.MarshalCanoto()
	block.bytes = blkBytes
	block.id = utils.ToID(blkBytes)
	return block, nil
}

func (b *StatelessBlock) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{B: bytes}
	return b.UnmarshalCanotoFrom(r)
}

// UnmarshalCanotoFrom overrides the embedded Block definition of the same function
// so that we can set the block bytes and ID fields.
func (b *StatelessBlock) UnmarshalCanotoFrom(r canoto.Reader) error {
	if err := b.Block.UnmarshalCanotoFrom(r); err != nil {
		return err
	}
	b.bytes = r.B
	b.id = utils.ToID(b.bytes)
	return nil
}

func (b *StatelessBlock) GetID() ids.ID        { return b.id }
func (b *StatelessBlock) GetBytes() []byte     { return b.bytes }
func (b *StatelessBlock) Size() int            { return len(b.bytes) }
func (b *StatelessBlock) GetStateRoot() ids.ID { return b.StateRoot }
func (b *StatelessBlock) GetHeight() uint64    { return b.Hght }
func (b *StatelessBlock) GetTimestamp() int64  { return b.Tmstmp }
func (b *StatelessBlock) GetParent() ids.ID    { return b.Prnt }
func (b *StatelessBlock) GetContext() *block.Context {
	return b.BlockContext
}

func (b *StatelessBlock) String() string {
	return fmt.Sprintf("(BlockID=%s, Height=%d, ParentRoot=%s, NumTxs=%d, Size=%d)", b.id, b.Hght, b.Prnt, len(b.Txs), len(b.bytes))
}

func UnmarshalBlock(raw []byte, parser TxParser) (*StatelessBlock, error) {
	r := canoto.Reader{
		B:       raw,
		Context: parser,
	}
	b := new(StatelessBlock)
	if err := b.UnmarshalCanotoFrom(r); err != nil {
		return nil, err
	}
	b.CalculateCanotoCache()
	return b, nil
}

func NewGenesisBlock(root ids.ID) (*ExecutionBlock, error) {
	// We set the genesis block timestamp to be after the ProposerVM fork activation.
	//
	// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
	// where the child timestamp is rounded down to the nearest second (which may be before
	// the timestamp of its parent, which is denoted in milliseconds).
	//
	// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
	// .../vms/proposervm/pre_fork_block.go#L201
	sb, err := NewStatelessBlock(
		ids.Empty,
		time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		0,
		nil,
		root, // StateRoot should include all allocates made when loading the genesis file
		nil,
	)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(sb), nil
}
