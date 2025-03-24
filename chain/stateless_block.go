// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"errors"
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"

	"github.com/ava-labs/hypersdk/utils"
)

var (
	_               canoto.Field = (*StatelessBlock)(nil)
	ErrNilTxInBlock              = errors.New("block contains nil transaction")
)

type Block struct {
	Prnt   ids.ID `canoto:"fixed bytes,1" json:"parent"`
	Tmstmp int64  `canoto:"fint64,2"      json:"timestamp"`
	Hght   uint64 `canoto:"fint64,3"      json:"height"`

	BlockContext *block.Context `canoto:"pointer,4" json:"blockContext"`

	// Note: canoto treats nil as a valid transaction. We only unmarshal
	// using the StatelessBlock type, which adds a check during unmarshal
	// to ensure there are no nil transactions.
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

// StatelessBlock defines the chain package raw block format.
// StatelessBlock must be treated as immutable.
//
// StatelessBlock implements [canoto.Field] and embeds [Block] to provide the
// default serialization and deserialization behavior. By overrdiing [canoto.Field],
// StatelessBlock caches the bytes and ID of the block to avoid re-computing them.
// This requires StatelessBlock to be treated as immutable, since modifying its fields
// will cause a divergence from the private, cached bytes and ID fields.
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

// UnmarshalCanoto overrides the embedded Block definition of the same function,
// so that we can make sure to set the block bytes and ID fields.
// Calling this function directly does not provide an opportunity to set the canoto
// reader Context to parse actions/auth contained in transactions, which means calling
// this function directly (instead of UnmarshalBlock) may fail for valid blocks.
// We implement it anyways because we prefer this function to fail on valid blocks rather
// than
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
	for i, tx := range b.Txs {
		if tx == nil {
			return fmt.Errorf("%w at index %d", ErrNilTxInBlock, i)
		}
	}
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

func UnmarshalBlock(raw []byte, parser Parser) (*StatelessBlock, error) {
	r := canoto.Reader{
		B:       raw,
		Context: parser,
	}
	b := new(StatelessBlock)
	if err := b.UnmarshalCanotoFrom(r); err != nil {
		return nil, err
	}
	// Call CalculateCanotoCache so that equivalent blocks pass an equals check.
	// Without calling this function, canoto's required internal field will cause equals
	// checks to fail on otherwise identical blocks.
	// TODO: remove after Canoto guarantees this to be set correctly on the read path (unmarshal)
	// rather than only setting it on the write path.
	b.CalculateCanotoCache()
	return b, nil
}
