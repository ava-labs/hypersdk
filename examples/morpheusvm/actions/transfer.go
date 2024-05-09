// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`
}

func (*Transfer) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *Transfer) StateKeys(actor codec.Address, _ codec.LID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}

func (*Transfer) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ codec.LID,
) (bool, uint64, [][]byte) {
	if t.Value == 0 {
		return false, 1, [][]byte{OutputValueZero}
	}
	if err := storage.SubBalance(ctx, mu, actor, t.Value); err != nil {
		return false, 1, [][]byte{utils.ErrBytes(err)}
	}
	if err := storage.AddBalance(ctx, mu, t.To, t.Value, true); err != nil {
		return false, 1, [][]byte{utils.ErrBytes(err)}
	}
	return true, 1, [][]byte{{}}
}

func (*Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer) Size() int {
	return codec.AddressLen + consts.Uint64Len
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackLID(t.To)
	p.PackUint64(t.Value)
}

func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer Transfer
	p.UnpackLID(true, &transfer.To) // we do not verify the typeID is valid
	transfer.Value = p.UnpackUint64(true)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return &transfer, nil
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
