// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
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

	size int

	// AuthCounts can be used by batch signature verification
	// to preallocate memory
	AuthCounts map[uint8]int
}

func (b *StatelessBlock) Size() int {
	return b.size
}

func (b *StatelessBlock) ID() (ids.ID, error) {
	blk, err := b.Marshal()
	if err != nil {
		return ids.ID{}, err
	}
	return utils.ToID(blk), nil
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
	b.AuthCounts = map[uint8]int{}
	for _, tx := range b.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
		b.AuthCounts[tx.Auth.GetTypeID()]++
	}

	p.PackID(b.StateRoot)
	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	b.size = len(bytes)
	return bytes, nil
}

func UnmarshalBlock(raw []byte, parser Parser) (*StatelessBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b StatelessBlock
	)
	b.size = len(raw)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	// Parse transactions
	txCount := p.UnpackInt(false) // can produce empty blocks
	actionCodec, authCodec := parser.ActionCodec(), parser.AuthCodec()
	b.Txs = []*Transaction{} // don't preallocate all to avoid DoS
	b.AuthCounts = map[uint8]int{}
	for i := uint32(0); i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionCodec, authCodec)
		if err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, tx)
		b.AuthCounts[tx.Auth.GetTypeID()]++
	}

	p.UnpackID(false, &b.StateRoot)

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &b, p.Err()
}

func NewGenesisBlock(root ids.ID) (*ExecutionBlock, error) {
	sb := &StatelessBlock{
		// We set the genesis block timestamp to be after the ProposerVM fork activation.
		//
		// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
		// where the child timestamp is rounded down to the nearest second (which may be before
		// the timestamp of its parent, which is denoted in milliseconds).
		//
		// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
		// .../vms/proposervm/pre_fork_block.go#L201
		Tmstmp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),

		// StateRoot should include all allocates made when loading the genesis file
		StateRoot: root,
	}

	sbBytes, err := sb.Marshal()
	if err != nil {
		return nil, err
	}
	id := utils.ToID(sbBytes)
	return &ExecutionBlock{
		StatelessBlock: sb,
		id:             id,
		bytes:          sbBytes,
	}, nil
}
