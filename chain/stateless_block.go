// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/window"
	"github.com/ava-labs/hypersdk/utils"
)

type StatelessBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	Txs []*Transaction `json:"txs"`

	// StateRoot is the root of the post-execution state
	// of [Prnt].
	//
	// This "deferred root" design allows for merklization
	// to be done asynchronously instead of during [Build]
	// or [Verify], which reduces the amount of time we are
	// blocking the consensus engine from voting on the block,
	// starting the verification of another block, etc.
	StateRoot ids.ID `json:"stateRoot"`

	bytes []byte
	id    ids.ID

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
	txsSet     set.Set[ids.ID]
}

func NewStatelessBlock(
	parentID ids.ID,
	timestamp int64,
	height uint64,
	txs []*Transaction,
	stateRoot ids.ID,
) (*StatelessBlock, error) {
	block := &StatelessBlock{
		Prnt:      parentID,
		Tmstmp:    timestamp,
		Hght:      height,
		Txs:       txs,
		StateRoot: stateRoot,
	}
	blkBytes, err := block.Marshal()
	if err != nil {
		return nil, err
	}
	block.bytes = blkBytes
	err = block.initTxsID()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (b *StatelessBlock) ID() ids.ID           { return b.id }
func (b *StatelessBlock) Bytes() []byte        { return b.bytes }
func (b *StatelessBlock) Size() int            { return len(b.bytes) }
func (b *StatelessBlock) GetStateRoot() ids.ID { return b.StateRoot }

func (b *StatelessBlock) String() string {
	return fmt.Sprintf("(BlockID=%s, Height=%d, ParentRoot=%s, Size=%d)", b.id, b.Hght, b.Prnt, len(b.bytes))
}

func (b *StatelessBlock) ContainsTx(id ids.ID) bool {
	return b.txsSet.Contains(id)
}

func (b *StatelessBlock) AuthCounts() map[uint8]int {
	return b.authCounts
}

func (b *StatelessBlock) Marshal() ([]byte, error) {
	size := ids.IDLen + consts.Uint64Len + consts.Uint64Len +
		consts.Uint64Len + window.WindowSliceSize +
		consts.IntLen + codec.CummSize(b.Txs) +
		ids.IDLen + consts.Uint64Len + consts.Uint64Len

	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackInt(uint32(len(b.Txs)))
	for _, tx := range b.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}

	p.PackID(b.StateRoot)
	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func (b *StatelessBlock) initTxsID() error {
	b.id = utils.ToID(b.bytes)
	b.authCounts = make(map[uint8]int)
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		if b.txsSet.Contains(tx.ID()) {
			return ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())
		b.authCounts[tx.Auth.GetTypeID()]++
	}
	return nil
}

func UnmarshalBlock(raw []byte, parser Parser) (*StatelessBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b StatelessBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	// Parse transactions
	txCount := p.UnpackInt(false) // can produce empty blocks
	actionCodec, authCodec := parser.ActionCodec(), parser.AuthCodec()
	b.Txs = []*Transaction{} // don't preallocate all to avoid DoS
	for i := uint32(0); i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionCodec, authCodec)
		if err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, tx)
	}

	p.UnpackID(false, &b.StateRoot)

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	b.bytes = raw
	err := b.initTxsID()
	if err != nil {
		return nil, err
	}
	return &b, p.Err()
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
	)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(sb), nil
}
